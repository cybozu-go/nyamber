package controllers

import (
	"context"
	"fmt"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testPodNamespace string = "default"
)

var _ = Describe("VirtualDC controller", func() {
	ctx := context.Background()
	var stopFunc func()

	BeforeEach(func() {
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:             scheme,
			LeaderElection:     false,
			MetricsBindAddress: "0",
		})
		Expect(err).ToNot(HaveOccurred())

		client := mgr.GetClient()
		nr := &VirtualDCReconciler{
			Client:            client,
			Scheme:            mgr.GetScheme(),
			PodNamespace:      testPodNamespace,
			JobProcessManager: NewJobProcessManager(ctrl.Log, client),
		}
		err = nr.SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: constants.Namespace,
			},
		}
		err = k8sClient.Create(ctx, ns)
		Expect(err).ToNot(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			err := mgr.Start(ctx)
			if err != nil {
				panic(err)
			}
		}()
		time.Sleep(100 * time.Millisecond)
	})

	AfterEach(func() {
		stopFunc()
		time.Sleep(100 * time.Millisecond)
	})

	It("should create pods and serivces for a virtualdc resource", func() {
		podTemplate := `apiVersion: v1
kind: Pod
spec:
  containers:
  - image: localhost:5151/entrypoint:dev
    imagePullPolicy: Always
    name: ubuntu
    command:
      - "/entrypoint"`
		cm := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: constants.Namespace,
				Name:      constants.PodTemplateName,
			},
			Data: map[string]string{"pod-template": podTemplate},
		}
		err := k8sClient.Create(ctx, cm)
		Expect(err).ToNot(HaveOccurred())

		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: "default",
			},
			Spec: nyamberv1beta1.VirtualDCSpec{
				NecoBranch:     "main",
				NecoAppsBranch: "main",
				Command:        []string{},
				// Resources: corev1.ResourceRequirements{},
			},
		}
		err = k8sClient.Create(ctx, vdc)
		Expect(err).ToNot(HaveOccurred())

		pod := &corev1.Pod{}
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-vdc", Namespace: "default"}, pod); err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		// check pod fields
		fmt.Println(pod.String())
	})
})
