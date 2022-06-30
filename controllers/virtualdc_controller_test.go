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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
	m.mu.Lock()
	defer m.mu.Unlock()

	vdcNamespacedName := types.NamespacedName{Namespace: vdc.Namespace, Name: vdc.Name}.String()
	delete(m.processes, vdcNamespacedName)
	return nil
}

func (m *mockJobProcessManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processes = map[string]struct{}{}
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

	It("should create and delete pods and services for a virtualdc resource", func() {
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
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
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
			"Args": Equal([]string{
				"neco_bootstrap:/neco-bootstrap",
				"neco_apps_bootstrap:/neco-apps-bootstrap",
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
		Expect(mock.processes).To(HaveKey(vdcNamespacedName))

		By("deleting vdc")
		err = k8sClient.Delete(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to delete a pod")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())

		By("checking to delete a service")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, svc)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())

		By("checking to stop jobProcessManager with correct arguments")
		Expect(mock.processes).NotTo(HaveKey(vdcNamespacedName))

		By("checking to delete a vdc")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, vdc)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
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
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		}).Should(Succeed())

		By("updating pod condtions")
		pod.Status.Conditions = append(pod.Status.Conditions,
			corev1.PodCondition{
				Type:   corev1.PodScheduled,
				Status: corev1.ConditionTrue,
				Reason: nyamberv1beta1.ReasonOK,
			}, corev1.PodCondition{
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
			if !meta.IsStatusConditionTrue(vdc.Status.Conditions, nyamberv1beta1.TypePodAvailable) {
				return fmt.Errorf("vdc status is expected to be PodAvailable, but actual %v", vdc.Status.Conditions)
			}
			return nil
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
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
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
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		}).Should(Succeed())

		By("checking to set Env and Args of pod")
		Expect(pod.Spec.Containers[0]).To(MatchFields(IgnoreExtras, Fields{
			"Env": ConsistOf([]corev1.EnvVar{
				{
					Name:  "NECO_BRANCH",
					Value: "main",
				},
			}),
			"Args": Equal([]string{"neco_bootstrap:/neco-bootstrap"}),
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
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		}).Should(Succeed())

		By("checking to set command of pod")
		Expect(pod.Spec.Containers[0].Args).To(Equal([]string{
			"neco_bootstrap:/neco-bootstrap",
			"neco_apps_bootstrap:/neco-apps-bootstrap",
			"user_defined_command:test command",
		}))
	})

	It("should not create a pod when the wrong configmap was created", func() {
		By("creating wrong configmap")
		cm := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: constants.ControllerNamespace, Name: constants.PodTemplateName}, cm)
		Expect(err).NotTo(HaveOccurred())

		podTemplate := `apiVersion: v1
kind: Pod
spec:
	containers:
	- image: entrypoint:envtest
	  name ubuntu`

		cm.Data = map[string]string{"pod-template": podTemplate}
		err = k8sClient.Update(ctx, cm)
		Expect(err).NotTo(HaveOccurred())

		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
		}
		err = k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to update vdc status")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testVdcNamespace}, vdc); err != nil {
				return err
			}
			if meta.IsStatusConditionFalse(vdc.Status.Conditions, nyamberv1beta1.TypePodCreated) {
				return nil
			}
			return fmt.Errorf("vdc status is expected to be PodCreated False, but actual %v", vdc.Status.Conditions)
		}).Should(Succeed())

		By("checking not to create pod")
		pod := &corev1.Pod{}
		err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		Expect(err).To(HaveOccurred())

		By("checking not to create svc")
		svc := &corev1.Service{}
		err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, svc)
		Expect(err).To(HaveOccurred())
	})

	It("should not create a pod when the wrong configmap that doesn't have Containers field", func() {
		By("creating wrong configmap that doesn't have Containers field")
		cm := &corev1.ConfigMap{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: constants.ControllerNamespace, Name: constants.PodTemplateName}, cm)
		Expect(err).NotTo(HaveOccurred())

		podTemplate := `apiVersion: v1
kind: Pod`

		cm.Data = map[string]string{"pod-template": podTemplate}
		err = k8sClient.Update(ctx, cm)
		Expect(err).NotTo(HaveOccurred())

		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
		}
		err = k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to update vdc status")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testVdcNamespace}, vdc); err != nil {
				return err
			}
			if meta.IsStatusConditionFalse(vdc.Status.Conditions, nyamberv1beta1.TypePodCreated) {
				return nil
			}
			return fmt.Errorf("vdc status is expected to be PodCreated False, but actual %v", vdc.Status.Conditions)
		}).Should(Succeed())

		By("checking not to create pod")
		pod := &corev1.Pod{}
		err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		Expect(err).To(HaveOccurred())

		By("checking not to create svc")
		svc := &corev1.Service{}
		err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, svc)
		Expect(err).To(HaveOccurred())
	})

	It("should recreate the service resource when the service resource is deleted", func() {
		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to create svc")
		svc := &corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, svc)
		}).Should(Succeed())

		uid := svc.GetUID()

		err = k8sClient.Delete(ctx, svc)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, svc); err != nil {
				return err
			}
			if uid == svc.GetUID() {
				return errors.New("the service resource is not recreated")
			}
			return nil
		}).Should(Succeed())
	})

	It("should not recreate the pod resource when the pod resource is deleted", func() {
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
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		}).Should(Succeed())

		err = k8sClient.Delete(ctx, pod)
		Expect(err).NotTo(HaveOccurred())

		By("checking to update vdc status")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testVdcNamespace}, vdc); err != nil {
				return err
			}
			if !meta.IsStatusConditionTrue(vdc.Status.Conditions, nyamberv1beta1.TypePodCreated) {
				return fmt.Errorf("vdc status is expected to be PodCreated True, but actual %v", vdc.Status.Conditions)
			}
			if !meta.IsStatusConditionFalse(vdc.Status.Conditions, nyamberv1beta1.TypePodAvailable) {
				return fmt.Errorf("vdc status is expected to be PodAvailable False, but actual %v", vdc.Status.Conditions)
			}
			return nil
		}).Should(Succeed())

		By("checking not to create pod")
		pod = &corev1.Pod{}
		err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		Expect(err).To(HaveOccurred())
	})

	It("should not create a pod when another pod with same name exists in same namespace", func() {
		By("creating a pod")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testPodNamespace,
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Image: "entrypoint:dev",
						Name:  "ubuntu",
					},
				},
			},
		}
		err := k8sClient.Create(ctx, pod)
		Expect(err).NotTo(HaveOccurred())

		pod.Status.Conditions = append(pod.Status.Conditions,
			corev1.PodCondition{
				Type:   corev1.PodScheduled,
				Status: corev1.ConditionTrue,
				Reason: nyamberv1beta1.ReasonOK,
			}, corev1.PodCondition{
				Type:   corev1.PodReady,
				Status: corev1.ConditionTrue,
				Reason: nyamberv1beta1.ReasonOK,
			})
		err = k8sClient.Status().Update(ctx, pod)
		Expect(err).NotTo(HaveOccurred())

		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
		}
		err = k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to update vdc status")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testVdcNamespace}, vdc); err != nil {
				return err
			}
			condPodCreated := meta.FindStatusCondition(vdc.Status.Conditions, nyamberv1beta1.TypePodCreated)
			if condPodCreated == nil {
				return fmt.Errorf("vdc condition is nil")
			}
			if condPodCreated.Status == metav1.ConditionTrue {
				return fmt.Errorf("vdc status is expected to be PodCreated False, but actual True")
			}
			if condPodCreated.Reason != nyamberv1beta1.ReasonPodCreatedConflict {
				return fmt.Errorf("vdc status reason is expected to be PodCreatedConflict, but actual %s", condPodCreated.Reason)
			}
			if !meta.IsStatusConditionFalse(vdc.Status.Conditions, nyamberv1beta1.TypePodAvailable) {
				return fmt.Errorf("vdc status is expected to be PodAvailable False, but actual %v", vdc.Status.Conditions)
			}
			return nil
		}).Should(Succeed())
	})

	It("should recreate the service resource when the service resource is deleted", func() {
		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to create svc")
		svc := &corev1.Service{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, svc)
		}).Should(Succeed())

		uid := svc.GetUID()

		err = k8sClient.Delete(ctx, svc)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, svc); err != nil {
				return err
			}
			if uid == svc.GetUID() {
				return errors.New("the service resource is not recreated")
			}
			return nil
		}).Should(Succeed())
	})

	It("should not recreate the pod resource when the pod resource is deleted", func() {
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
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		}).Should(Succeed())

		err = k8sClient.Delete(ctx, pod)
		Expect(err).NotTo(HaveOccurred())

		By("checking to update vdc status")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testVdcNamespace}, vdc); err != nil {
				return err
			}
			if !meta.IsStatusConditionTrue(vdc.Status.Conditions, nyamberv1beta1.TypePodCreated) {
				return fmt.Errorf("vdc status is expected to be PodCreated True, but actual %v", vdc.Status.Conditions)
			}
			if !meta.IsStatusConditionFalse(vdc.Status.Conditions, nyamberv1beta1.TypePodAvailable) {
				return fmt.Errorf("vdc status is expected to be PodAvailable False, but actual %v", vdc.Status.Conditions)
			}
			return nil
		}).Should(Succeed())

		By("checking not to create pod")
		pod = &corev1.Pod{}
		err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testPodNamespace}, pod)
		Expect(err).To(HaveOccurred())
	})

	It("should not create a pod when another pod with same name exists in same namespace", func() {
		By("creating a service")
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testPodNamespace,
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					constants.LabelKeyOwnerNamespace: testVdcNamespace,
					constants.LabelKeyOwner:          "test-vdc",
				},
				Ports: []corev1.ServicePort{
					{
						Name:       "status",
						Protocol:   corev1.ProtocolTCP,
						Port:       80,
						TargetPort: intstr.FromInt(constants.ListenPort),
					},
				},
			},
		}
		err := k8sClient.Create(ctx, svc)
		Expect(err).NotTo(HaveOccurred())

		By("creating a VirtualDC resource")
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
		}
		err = k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to update vdc status")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: testVdcNamespace}, vdc); err != nil {
				return err
			}
			condServiceCreated := meta.FindStatusCondition(vdc.Status.Conditions, nyamberv1beta1.TypeServiceCreated)
			if condServiceCreated == nil {
				return fmt.Errorf("vdc condition is nil")
			}
			if condServiceCreated.Status == metav1.ConditionTrue {
				return fmt.Errorf("vdc status is expected to be ServiceCreated False, but actual True")
			}
			if condServiceCreated.Reason != nyamberv1beta1.ReasonServiceCreatedConflict {
				return fmt.Errorf("vdc status reason is expected to be Conflict, but actual %s", condServiceCreated.Reason)
			}
			return nil
		}).Should(Succeed())

		By("checking not to update service resource")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(svc), svc); err != nil {
				return err
			}
			if svc.Labels[constants.LabelKeyOwnerNamespace] == testVdcNamespace {
				return fmt.Errorf("OwnerNameSpace label is expected to nil, but actual %s", testVdcNamespace)
			}
			if svc.Labels[constants.LabelKeyOwner] == testVdcNamespace {
				return fmt.Errorf("Owner label is expected to nil, but actual %s", "test-vdc")
			}
			return nil
		}).Should(Succeed())
	})
})
