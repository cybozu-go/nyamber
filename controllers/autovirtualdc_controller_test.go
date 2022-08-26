package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type MockClock struct {
	now time.Time
}

func (m *MockClock) Now() time.Time {
	return m.now
}

var _ = Describe("AutoVirtualDC controller", func() {
	ctx := context.Background()
	var stopFunc func()
	clock := &MockClock{now: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)}

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
			Clock:  clock,
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
			Kind:               "AutoVirtualDC",
			APIVersion:         "nyamber.cybozu.io/v1beta1",
			UID:                avdc.UID,
			Name:               avdc.Name,
			Controller:         pointer.Bool(true),
			BlockOwnerDeletion: pointer.Bool(true),
		}
		Expect(vdc.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
	})

	It("should have status according to its schedule", func() {
		type input struct {
			now        time.Time
			conditions []metav1.Condition
		}
		testcases := []struct {
			name     string
			input    input
			expected nyamberv1beta1.Operation
		}{
			{
				name: "before startTime after stopTime",
				input: input{
					now:        time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
					conditions: nil,
				},
				expected: nyamberv1beta1.Operation{
					Name: nyamberv1beta1.Start,
					Time: metav1.NewTime(time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC)),
				},
			},
			{
				name: "after startTime before stopTime",
				input: input{
					now: time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC),
					conditions: []metav1.Condition{
						{
							Type:   nyamberv1beta1.TypePodJobCompleted,
							Status: metav1.ConditionTrue,
							Reason: nyamberv1beta1.ReasonOK,
						},
					},
				},
				expected: nyamberv1beta1.Operation{
					Name: nyamberv1beta1.Stop,
					Time: metav1.NewTime(time.Date(2000, 1, 1, 5, 0, 0, 0, time.UTC)),
				},
			},
			{
				name: "after startTime before stopTime (pod job is not completed)",
				input: input{
					now: time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC),
					conditions: []metav1.Condition{
						{
							Type:   nyamberv1beta1.TypePodJobCompleted,
							Status: metav1.ConditionFalse,
							Reason: nyamberv1beta1.ReasonPodJobCompletedRunning,
						},
					},
				},
				expected: nyamberv1beta1.Operation{
					Name: nyamberv1beta1.Start,
					Time: metav1.NewTime(time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC)),
				},
			},
		}

		for _, testcase := range testcases {
			By(fmt.Sprintf("creating AutoVirtualDC with schedule: %s", testcase.name))
			clock.now = testcase.input.now
			avdc := &nyamberv1beta1.AutoVirtualDC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-avdc",
					Namespace: testNamespace,
				},
				Spec: nyamberv1beta1.AutoVirtualDCSpec{
					StartSchedule: "0 1 * * *",
					StopSchedule:  "0 5 * * *",
				},
			}
			err := k8sClient.Create(ctx, avdc)
			Expect(err).NotTo(HaveOccurred())

			if testcase.input.conditions != nil {
				vdc := &nyamberv1beta1.VirtualDC{}
				Eventually(func() error {
					return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
				}).Should(Succeed())

				for _, condition := range testcase.input.conditions {
					meta.SetStatusCondition(&vdc.Status.Conditions, condition)
				}
				err := k8sClient.Status().Update(ctx, vdc)
				Expect(err).NotTo(HaveOccurred())
			}

			By("checking NextOperation")
			var operation *nyamberv1beta1.Operation

			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, avdc)
				g.Expect(err).NotTo(HaveOccurred())
				operation = avdc.Status.NextOperation
				g.Expect(operation).NotTo(BeNil())
				g.Expect(operation.Name).To(Equal(testcase.expected.Name))
				expectTime := testcase.expected.Time
				g.Expect(operation.Time.Equal(&expectTime))
			}).Should(Succeed())

			By("deleting AutoVirtualDC")
			err = k8sClient.Delete(ctx, avdc)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, avdc)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())
		}
	})
})
