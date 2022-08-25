package controllers

import (
	"context"
	"errors"

	"time"
	"k8s.io/utils/pointer"
	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("AutoVirtualDC controller", func() {
	ctx := context.Background()
	var stopFunc func()
	BeforeEach(func() {
		time.Sleep(100 * time.Millisecond)

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:             scheme,
			LeaderElection:     false,
			MetricsBindAddress: "0",
		})
		Expect(err).NotTo(HaveOccurred())

		client := mgr.GetClient()
		nr := &AutoVirtualDCReconciler{
			Client: client,
			Scheme: mgr.GetScheme(),
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
	})

	AfterEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &nyamberv1beta1.AutoVirtualDC{}, client.InNamespace(testNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &nyamberv1beta1.VirtualDC{}, client.InNamespace(testNamespace))
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() error {
			avdcs := &nyamberv1beta1.AutoVirtualDCList{}
			if err := k8sClient.List(ctx, avdcs, client.InNamespace(testNamespace)); err != nil {
				return err
			}
			if len(avdcs.Items) != 0 {
				return errors.New("avdcs is not deleted")
			}
			vdcs := &nyamberv1beta1.VirtualDCList{}
			if err := k8sClient.List(ctx, vdcs, client.InNamespace(testNamespace)); err != nil {
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

	It("should create and delete virtualDC", func() {
		By("creating an AutoVirtualDC resource")
		avdc := &nyamberv1beta1.AutoVirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-avdc",
				Namespace: testNamespace,
			},
		}
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking to add finalizer")
		Eventually(func() error {
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, avdc); err != nil {
				return err
			}
			for _, elm := range avdc.ObjectMeta.Finalizers {
				if elm == constants.FinalizerName {
					return nil
				}
			}
			return errors.New("finalizer is not found")
		}).Should(Succeed())

		By("checking to create virtualDC")
		vdc := &nyamberv1beta1.VirtualDC{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())
		By("checking if virtualDC has OwnerReference")

		expectedOwnerReference := metav1.OwnerReference{
			Kind:       "AutoVirtualDC",
			APIVersion: "nyamber.cybozu.io/v1beta1",
			UID:        avdc.UID,
			Name:       avdc.Name,
			Controller: pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		}
		Expect(vdc.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))

	})
})

