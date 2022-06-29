package controllers

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mockJobProcessManager struct {
	mu        sync.Mutex
	stopped   bool
	processes map[string]struct{}
}

func (m *mockJobProcessManager) Start(vdc *nyamberv1beta1.VirtualDC) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return errors.New("JobProcessManager is already stopped")
	}

	vdcNamespacedName := types.NamespacedName{Namespace: vdc.Namespace, Name: vdc.Name}.String()
	m.processes[vdcNamespacedName] = struct{}{}

	return nil
}

func (m *mockJobProcessManager) Stop(vdc *nyamberv1beta1.VirtualDC) error {
	return nil
}

func (m *mockJobProcessManager) StopAll() {
}

var _ = Describe("VirtualDC controller", func() {
	ctx := context.Background()
	var stopFunc func()
	mock := mockJobProcessManager{
		mu:        sync.Mutex{},
		processes: make(map[string]struct{}),
	}

	BeforeEach(func() {
		time.Sleep(100 * time.Millisecond)

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:             scheme,
			LeaderElection:     false,
			MetricsBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred())

		client := mgr.GetClient()
		nr := &VirtualDCReconciler{
			Client:            client,
			Scheme:            mgr.GetScheme(),
			PodNamespace:      testPodNamespace,
			JobProcessManager: &mock,
		}
		err = nr.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		cctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			err := mgr.Start(cctx)
			if err != nil {
				panic(err)
			}
		}()
		time.Sleep(100 * time.Millisecond)

		podTemplate := `apiVersion: v1
kind: Pod
spec:
  containers:
  - image: entrypoint:envtest
    name: ubuntu`

		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: constants.ControllerNamespace,
				Name:      constants.PodTemplateName,
			},
			Data: map[string]string{"pod-template": podTemplate},
		}
		err = k8sClient.Create(ctx, cm)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &corev1.ConfigMap{}, client.InNamespace(constants.ControllerNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(testPodNamespace))
		Expect(err).NotTo(HaveOccurred())
		svcs := &corev1.ServiceList{}
		err = k8sClient.List(ctx, svcs, client.InNamespace(testPodNamespace))
		Expect(err).NotTo(HaveOccurred())
		for _, svc := range svcs.Items {
			err := k8sClient.Delete(ctx, &svc)
			Expect(err).NotTo(HaveOccurred())
		}
		err = k8sClient.DeleteAllOf(ctx, &nyamberv1beta1.VirtualDC{}, client.InNamespace(testVdcNamespace))
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() error {
			vdcs := &nyamberv1beta1.VirtualDCList{}
			if err := k8sClient.List(ctx, vdcs, client.InNamespace(testVdcNamespace)); err != nil {
				return err
			}
			if len(vdcs.Items) != 0 {
				return errors.New("vdcs is not deleted")
			}
			return nil
		}).Should(Succeed())
		time.Sleep(100 * time.Millisecond)
		stopFunc()
		time.Sleep(100 * time.Millisecond)
	})

	It("should create pods and services for a virtualdc resource", func() {
		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to add finalizer")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testVdcNamespace}, vdc); err != nil {
				return err
			}
			for _, elm := range vdc.ObjectMeta.Finalizers {
				if elm == constants.FinalizerName {
					return nil
				}
			}
			return errors.New("finalizer is not found")
		}).Should(Succeed())

		By("checking to create pod")
		pod := &corev1.Pod{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(pod.Labels).To(MatchAllKeys(Keys{
			constants.LabelKeyOwnerNamespace: Equal(testVdcNamespace),
			constants.LabelKeyOwner:          Equal("test-vdc"),
		}))
		Expect(pod.Spec.Containers).To(HaveLen(1))
		Expect(pod.Spec.Containers[0]).To(MatchFields(IgnoreExtras, Fields{
			"Image": Equal("entrypoint:envtest"),
			"Name":  Equal("ubuntu"),
			"Env": ConsistOf([]corev1.EnvVar{
				{
					Name:  "NECO_BRANCH",
					Value: "main",
				},
				{
					Name:  "NECO_APPS_BRANCH",
					Value: "main",
				},
			}),
			"Args": MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": Equal("neco_bootstrap:/neco-bootstrap"),
				"1": Equal("neco_apps_bootstrap:/neco-apps-bootstrap"),
			}),
		}))

		By("checking to create svc")
		svc := &corev1.Service{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, svc); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Expect(svc.Labels).To(MatchAllKeys(Keys{
			constants.LabelKeyOwnerNamespace: Equal(testVdcNamespace),
			constants.LabelKeyOwner:          Equal("test-vdc"),
		}))
		Expect(svc.Spec).To(MatchFields(IgnoreExtras, Fields{
			"Selector": MatchAllKeys(Keys{
				constants.LabelKeyOwnerNamespace: Equal(testVdcNamespace),
				constants.LabelKeyOwner:          Equal("test-vdc"),
			}),
			"Ports": ConsistOf([]corev1.ServicePort{
				{
					Name:       "status",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(constants.ListenPort),
				},
			}),
		}))

		By("checking to call JobProcessManager.Start")
		vdcNamespacedName := types.NamespacedName{Namespace: vdc.Namespace, Name: vdc.Name}.String()
		_, ok := mock.processes[vdcNamespacedName]
		Expect(ok).To(BeTrue())
	})

	It("should change status based on pod status", func() {
		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to create pod")
		pod := &corev1.Pod{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		By("updating pod condtions")
		pod.Status.Conditions = append(pod.Status.Conditions,
			corev1.PodCondition{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
				Reason: nyamberv1beta1.ReasonOK,
			})
		err = k8sClient.Status().Update(ctx, pod)
		Expect(err).NotTo(HaveOccurred())

		By("checking to change vdc status")
		Eventually(func() error {
			vdc := &nyamberv1beta1.VirtualDC{}
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testVdcNamespace}, vdc); err != nil {
				return err
			}
			for _, cond := range vdc.Status.Conditions {
				if cond.Type == nyamberv1beta1.TypePodAvailable && cond.Status == metav1.ConditionTrue {
					return nil
				}
			}
			return fmt.Errorf("vdc status is expected to be PodAvailable, but acutal %v", vdc.Status.Conditions)
		}).Should(Succeed())
	})

	It("should create a pod with correct branch name set by VirtualDC spec", func() {
		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
			Spec: nyamberv1beta1.VirtualDCSpec{
				NecoBranch:     "test-neco-branch",
				NecoAppsBranch: "test-neco-apps-branch",
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to create pod")
		pod := &corev1.Pod{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		By("checking to set Env")
		Expect(pod.Spec.Containers[0].Env).To(ConsistOf([]corev1.EnvVar{
			{
				Name:  "NECO_BRANCH",
				Value: vdc.Spec.NecoBranch,
			},
			{
				Name:  "NECO_APPS_BRANCH",
				Value: vdc.Spec.NecoAppsBranch,
			},
		}))
	})

	It("should create a pod with correct command when VirtualDC resource set SkipNecoApps true", func() {
		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
			Spec: nyamberv1beta1.VirtualDCSpec{
				SkipNecoApps: true,
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to create pod")
		pod := &corev1.Pod{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		By("checking to set Env and Args of pod")
		Expect(pod.Spec.Containers[0]).To(MatchFields(IgnoreExtras, Fields{
			"Env": ConsistOf([]corev1.EnvVar{
				{
					Name:  "NECO_BRANCH",
					Value: "main",
				},
			}),
			"Args": MatchAllElementsWithIndex(IndexIdentity, Elements{
				"0": Equal("neco_bootstrap:/neco-bootstrap"),
			}),
		}))
	})

	It("should create a pod with correct command when the user specifies command", func() {
		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
			Spec: nyamberv1beta1.VirtualDCSpec{
				Command: []string{"test", "command"},
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to create pod")
		pod := &corev1.Pod{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())

		By("checking to set command of pod")
		Expect(pod.Spec.Containers[0].Args).To(ConsistOf([]string{
			"neco_bootstrap:/neco-bootstrap",
			"neco_apps_bootstrap:/neco-apps-bootstrap",
			"user_defined_command:test command",
		}))
	})
})
