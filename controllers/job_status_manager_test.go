package controllers

import (
	"context"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var _ = Describe("Job watch process", func() {
	ctx := context.Background()

	AfterEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &nyamberv1beta1.VirtualDC{}, client.InNamespace(testNamespace))
		Expect(err).NotTo(HaveOccurred())
	})

	createVdc := func(conditions []metav1.Condition) *nyamberv1beta1.VirtualDC {
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc-job",
				Namespace: testNamespace,
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())
		if len(conditions) != 0 {
			for _, cond := range conditions {
				meta.SetStatusCondition(&vdc.Status.Conditions, cond)
			}
			err = k8sClient.Status().Update(ctx, vdc)
			Expect(err).NotTo(HaveOccurred())
		}
		return vdc
	}

	It("should mark the job as failed when the created pod does not exist", func() {
		By("creating a VirtualDC whose pod has been created")
		vdc := createVdc([]metav1.Condition{{
			Type:   nyamberv1beta1.TypePodCreated,
			Status: metav1.ConditionTrue,
			Reason: nyamberv1beta1.ReasonOK,
		}})

		By("updating the status without the pod")
		p := newJobWatchProcess(logf.Log, k8sClient, vdc, testPodNamespace)
		retry, err := p.updateStatus(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(retry).To(BeFalse())

		By("checking the job condition")
		err = k8sClient.Get(ctx, client.ObjectKeyFromObject(vdc), vdc)
		Expect(err).NotTo(HaveOccurred())
		cond := meta.FindStatusCondition(vdc.Status.Conditions, nyamberv1beta1.TypePodJobCompleted)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal(nyamberv1beta1.ReasonPodJobCompletedFailed))
	})

	It("should not mark the job as failed when the pod has not been created", func() {
		By("creating a VirtualDC whose pod has not been created")
		vdc := createVdc(nil)

		By("updating the status without the pod")
		p := newJobWatchProcess(logf.Log, k8sClient, vdc, testPodNamespace)
		retry, err := p.updateStatus(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(retry).To(BeFalse())

		By("checking the job condition")
		err = k8sClient.Get(ctx, client.ObjectKeyFromObject(vdc), vdc)
		Expect(err).NotTo(HaveOccurred())
		Expect(meta.FindStatusCondition(vdc.Status.Conditions, nyamberv1beta1.TypePodJobCompleted)).To(BeNil())
	})
})
