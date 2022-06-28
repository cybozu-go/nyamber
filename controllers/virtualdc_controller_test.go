package controllers

import (
	"context"
	"errors"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testVdcNamespace string = "test-vdc-ns"
	testPodNamespace string = "test-pod-ns"
)

var _ = Describe("VirtualDC controller", func() {
	ctx := context.Background()
	var (
		vdcNs *corev1.Namespace
		podNs *corev1.Namespace
	)
	var stopFunc func()

	BeforeEach(func() {
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
			JobProcessManager: NewJobProcessManager(ctrl.Log, client),
		}
		err = nr.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		vdcNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testVdcNamespace,
			},
		}
		err = k8sClient.Create(context.Background(), vdcNs)
		Expect(err).NotTo(HaveOccurred())
		podNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testPodNamespace,
			},
		}
		err = k8sClient.Create(context.Background(), podNs)
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
	})

	AfterEach(func() {
		stopFunc()
		err := k8sClient.Delete(ctx, vdcNs)
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.Delete(ctx, podNs)
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(100 * time.Millisecond)
	})

	It("should create pods and services for a virtualdc resource", func() {
		By("creating configmap for pod template")
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
		err := k8sClient.Create(ctx, cm)
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

	})
})
