package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

type MockClock struct {
	*testing.FakeClock
}

func (m *MockClock) Sub(a, b time.Time) time.Duration {
	return time.Second
}

var _ = Describe("AutoVirtualDC controller", func() {
	ctx := context.Background()
	var stopFunc func()
	clock := &MockClock{FakeClock: testing.NewFakeClock(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))}

	BeforeEach(func() {
		time.Sleep(100 * time.Millisecond)

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme:         scheme,
			LeaderElection: false,
			Metrics:        metricsserver.Options{BindAddress: "0"},
			Controller: config.Controller{
				SkipNameValidation: ptr.To(true),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		client := mgr.GetClient()
		nr := &AutoVirtualDCReconciler{
			Client:          client,
			Scheme:          mgr.GetScheme(),
			Clock:           clock,
			RequeueInterval: time.Second,
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
			Controller:         ptr.To[bool](true),
			BlockOwnerDeletion: ptr.To[bool](true),
		}
		Expect(vdc.ObjectMeta.OwnerReferences).To(ContainElement(expectedOwnerReference))
	})

	It("should have status according to its schedule", func() {
		type input struct {
			now        time.Time
			conditions []metav1.Condition
		}
		testcases := []struct {
			name                  string
			input                 input
			expectedNextStartTime metav1.Time
			expectedNextStopTime  metav1.Time
		}{
			{
				name: "before startTime after stopTime (case 1)",
				input: input{
					now:        time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
					conditions: nil,
				},
				expectedNextStartTime: metav1.NewTime(time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC)),
				expectedNextStopTime:  metav1.NewTime(time.Date(2000, 1, 1, 5, 0, 0, 0, time.UTC)),
			},
			{
				name: "after startTime before stopTime (case 2 to 1)",
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
				expectedNextStartTime: metav1.NewTime(time.Date(2000, 1, 2, 1, 0, 0, 0, time.UTC)),
				expectedNextStopTime:  metav1.NewTime(time.Date(2000, 1, 1, 5, 0, 0, 0, time.UTC)),
			},
			{
				name: "after startTime before stopTime (pod job is not completed) (case 2 loop)",
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
				expectedNextStartTime: metav1.NewTime(time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC)),
				expectedNextStopTime:  metav1.NewTime(time.Date(2000, 1, 1, 5, 0, 0, 0, time.UTC)),
			},
		}

		for _, testcase := range testcases {
			By(fmt.Sprintf("creating AutoVirtualDC with schedule: %s", testcase.name))
			clock.SetTime(testcase.input.now)
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

			By("checking VirtualDC is created and its condition is correct")
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

			By("checking status is expected")
			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, avdc)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(avdc.Status.NextStartTime).NotTo(BeNil())
				g.Expect(avdc.Status.NextStartTime.Time).To(BeTemporally("==", testcase.expectedNextStartTime.Time))
				g.Expect(avdc.Status.NextStopTime).NotTo(BeNil())
				g.Expect(avdc.Status.NextStopTime.Time).To(BeTemporally("==", testcase.expectedNextStopTime.Time))
			}).Should(Succeed())

			By("deleting AutoVirtualDC")
			err = k8sClient.Delete(ctx, avdc)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, avdc)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())

			By("deleting VirtualDC")
			if testcase.input.conditions != nil {
				vdc := &nyamberv1beta1.VirtualDC{}
				Eventually(func() error {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
					return err
				}).ShouldNot(HaveOccurred())
				err = k8sClient.Delete(ctx, vdc)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
					return apierrors.IsNotFound(err)
				}).Should(BeTrue())
			}
		}
	})

	It("should operate VDC according to its status without schedule", func() {
		By("creating AutoVirtualDC")
		clock.SetTime(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
		avdc := &nyamberv1beta1.AutoVirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-avdc",
				Namespace: testNamespace,
			},
		}
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking VirtualDC is created")
		clock.Step(time.Second)
		vdc := &nyamberv1beta1.VirtualDC{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())
		previousVdcUid := vdc.UID

		By("checking vdc is not recreated")
		clock.Step(time.Second)
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())

		By("setting vdc's status to be failed")
		clock.Step(time.Second)
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedFailed,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is recreated")
		clock.Step(time.Second)
		vdc = &nyamberv1beta1.VirtualDC{}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).NotTo(Equal(previousVdcUid))
		}).Should(Succeed())
		previousVdcUid = vdc.UID

		By("setting vdc's status to be pending")
		clock.Step(time.Second)
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedPending,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is not recreated")
		clock.Step(time.Second)
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())

		By("setting vdc's status to be completed")
		clock.Step(time.Second)
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionTrue,
			Reason: nyamberv1beta1.ReasonOK,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is not recreated")
		clock.Step(time.Second)
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())
	})

	It("should operate VDC according to its status with schedule. creating avdc between startTime and stopTime.", func() {
		By("creating AutoVirtualDC on time that is between start and stop")
		clock.SetTime(time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC))
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

		By("checking VirtualDC is created")
		clock.Step(time.Second)
		vdc := &nyamberv1beta1.VirtualDC{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())
		previousVdcUid := vdc.UID

		By("checking vdc is not recreated")
		clock.Step(time.Second)
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())

		By("setting vdc's status to be failed")
		clock.Step(time.Second)
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedFailed,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is recreated")
		clock.Step(time.Second)
		vdc = &nyamberv1beta1.VirtualDC{}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).NotTo(Equal(previousVdcUid))
		}).Should(Succeed())
		previousVdcUid = vdc.UID

		By("setting vdc's status to be pending")
		clock.Step(time.Second)
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedPending,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is not recreated")
		clock.Step(time.Second)
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())

		By("setting vdc's status to be completed")
		clock.Step(time.Second)
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionTrue,
			Reason: nyamberv1beta1.ReasonOK,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is not recreated")
		clock.Step(time.Second)
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())

		By("checking vdc will be deleted on stop time")
		clock.SetTime(time.Date(2000, 1, 1, 5, 0, 0, 0, time.UTC))
		Eventually(func() bool {
			vdc := &nyamberv1beta1.VirtualDC{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())

		By("checking vdc will be created on start time")
		clock.SetTime(time.Date(2000, 1, 2, 1, 0, 0, 0, time.UTC))
		Eventually(func() error {
			vdc := &nyamberv1beta1.VirtualDC{}
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())
	})

	It("should operate VDC according to its status with schedule, creating AVDC  between stopTime and startTime", func() {
		By("creating AutoVirtualDC between stopTime and startTime")
		clock.SetTime(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
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

		By("checking VirtualDC is not created")
		Eventually(func() bool {
			vdc := &nyamberv1beta1.VirtualDC{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())

		By("checking VirtualDC is created")
		clock.SetTime(time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC))
		vdc := &nyamberv1beta1.VirtualDC{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())
		previousVdcUid := vdc.UID

		By("checking vdc is not recreated")
		clock.Step(time.Second)
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())
	})

	It("should delete VDC if VDC's status keeps pending but stopTime has come.", func() {
		By("creating AutoVirtualDC between startTime and stopTime")
		clock.SetTime(time.Date(2000, 1, 1, 2, 0, 0, 0, time.UTC))
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

		By("checking VirtualDC is created")
		vdc := &nyamberv1beta1.VirtualDC{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())

		By("setting vdc's status to be pending")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedPending,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc deleted when stopTime comes")
		clock.SetTime(time.Date(2000, 1, 1, 5, 0, 0, 0, time.UTC))
		Eventually(func() bool {
			vdc := &nyamberv1beta1.VirtualDC{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	})

	It("should operate VDC according to TimeoutDuration of AVDC", func() {
		By("creating AutoVirtualDC between startTime and stopTime")
		clock.SetTime(time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC))
		avdc := &nyamberv1beta1.AutoVirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-avdc",
				Namespace: testNamespace,
			},
			Spec: nyamberv1beta1.AutoVirtualDCSpec{
				StartSchedule:   "0 1 * * *",
				StopSchedule:    "0 5 * * *",
				TimeoutDuration: "1h",
			},
		}
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking VirtualDC is created")
		vdc := &nyamberv1beta1.VirtualDC{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())
		previousVdcUid := vdc.UID

		By("setting vdc's status to be failed")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedFailed,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is recreated when vdc failed")
		Eventually(func(g Gomega) {
			vdc = &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).NotTo(Equal(previousVdcUid))
		}).Should(Succeed())
		previousVdcUid = vdc.UID

		By("Setting the time to the time when the timeout has expired")
		clock.SetTime(time.Date(2000, 1, 1, 2, 0, 0, 1, time.UTC))

		By("setting vdc's status to be failed")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedFailed,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is not recreated if timeout has passed")
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())

		By("checking vdc deleted when stopTime comes")
		clock.SetTime(time.Date(2000, 1, 1, 5, 0, 0, 0, time.UTC))
		Eventually(func() bool {
			vdc := &nyamberv1beta1.VirtualDC{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	})

	It("should not recreate VDC with TimeoutDuration 0", func() {
		By("creating AutoVirtualDC between startTime and stopTime")
		clock.SetTime(time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC))
		avdc := &nyamberv1beta1.AutoVirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-avdc",
				Namespace: testNamespace,
			},
			Spec: nyamberv1beta1.AutoVirtualDCSpec{
				StartSchedule:   "0 1 * * *",
				StopSchedule:    "0 5 * * *",
				TimeoutDuration: "0s",
			},
		}
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking VirtualDC is created")
		vdc := &nyamberv1beta1.VirtualDC{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())
		previousVdcUid := vdc.UID

		By("setting vdc's status to be failed")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedFailed,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is recreated when vdc failed")
		Eventually(func(g Gomega) {
			vdc = &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).NotTo(Equal(previousVdcUid))
		}).Should(Succeed())
		previousVdcUid = vdc.UID

		By("Setting the time to the time when the timeout has expired")
		clock.SetTime(time.Date(2000, 1, 1, 1, 0, 0, 1, time.UTC))

		By("setting vdc's status to be failed")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedFailed,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is not recreated if timeout has passed")
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())

		By("checking vdc deleted when stopTime comes")
		clock.SetTime(time.Date(2000, 1, 1, 5, 0, 0, 0, time.UTC))
		Eventually(func() bool {
			vdc := &nyamberv1beta1.VirtualDC{}
			err := k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			return apierrors.IsNotFound(err)
		}).Should(BeTrue())
	})

	It("should operate VDC according to TimeoutDuration of AVDC when startSchedule/stopSchedule is not set", func() {
		By("creating AutoVirtualDC")
		clock.SetTime(time.Date(2000, 1, 1, 1, 0, 0, 0, time.UTC))
		avdc := &nyamberv1beta1.AutoVirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-avdc",
				Namespace: testNamespace,
			},
			Spec: nyamberv1beta1.AutoVirtualDCSpec{
				TimeoutDuration: "30s",
			},
		}
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking VirtualDC is created")
		vdc := &nyamberv1beta1.VirtualDC{}
		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
		}).Should(Succeed())
		previousVdcUid := vdc.UID

		By("setting vdc's status to be failed")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedFailed,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is recreated when vdc failed")
		Eventually(func(g Gomega) {
			vdc = &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).NotTo(Equal(previousVdcUid))
		}).Should(Succeed())
		previousVdcUid = vdc.UID

		By("Setting the time to the time when the timeout has expired")
		clock.SetTime(avdc.CreationTimestamp.Time.Add(2 * time.Hour))

		By("setting vdc's status to be failed")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodJobCompleted,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodJobCompletedFailed,
		})
		err = k8sClient.Status().Update(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("checking vdc is not recreated if timeout has passed")
		Consistently(func(g Gomega) {
			vdc := &nyamberv1beta1.VirtualDC{}
			err = k8sClient.Get(ctx, client.ObjectKey{Name: "test-avdc", Namespace: testNamespace}, vdc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(vdc.UID).To(Equal(previousVdcUid))
		}).Should(Succeed())

	})
})
