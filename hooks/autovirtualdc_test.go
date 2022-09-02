package hooks

import (
	"context"
	"errors"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("AutoVirtualDC validator", func() {
	ctx := context.Background()

	BeforeEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &nyamberv1beta1.AutoVirtualDC{}, client.InNamespace(testNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &nyamberv1beta1.VirtualDC{}, client.InNamespace(testNamespace))
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			avdcs := &nyamberv1beta1.AutoVirtualDCList{}
			if err := k8sClient.List(ctx, avdcs); err != nil {
				return err
			}
			if len(avdcs.Items) != 0 {
				return errors.New("avdcs is not deleted")
			}
			vdcs := &nyamberv1beta1.VirtualDCList{}
			if err := k8sClient.List(ctx, vdcs); err != nil {
				return err
			}
			if len(vdcs.Items) != 0 {
				return errors.New("vdcs is not deleted")
			}
			return nil
		}).Should(Succeed())
	})

	It("should allow to create autovirtualdc resources if the manifest is valid", func() {
		avdc := makeAutoVirtualDC()
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should deny to create autoVirtualDC resources", func() {
		testcases := []struct {
			name string
			spec nyamberv1beta1.AutoVirtualDCSpec
		}{
			{
				"only startSchedule is blank",
				nyamberv1beta1.AutoVirtualDCSpec{
					StartSchedule: "",
					StopSchedule:  "0 5 * * *",
				},
			},
			{
				"only stopSchedule is blank",
				nyamberv1beta1.AutoVirtualDCSpec{
					StartSchedule: "0 2 * * *",
					StopSchedule:  "",
				},
			},
			{
				"startSchedule can not be parsed",
				nyamberv1beta1.AutoVirtualDCSpec{
					StartSchedule: "0 0",
					StopSchedule:  "0 2 * * *",
				},
			},
			{
				"stopSchedule can not be parsed",
				nyamberv1beta1.AutoVirtualDCSpec{
					StartSchedule: "0 5 * * *",
					StopSchedule:  "0 hoge * * *",
				},
			},
			{
				"timeoutDuration can not be parsed",
				nyamberv1beta1.AutoVirtualDCSpec{
					TimeoutDuration: "hoge",
				},
			},
		}

		for _, testcase := range testcases {
			By(testcase.name)
			avdc := &nyamberv1beta1.AutoVirtualDC{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-avdc",
					Namespace: testNamespace,
				},
				Spec: testcase.spec,
			}
			err := k8sClient.Create(ctx, avdc)
			Expect(err).To(HaveOccurred())
		}
	})

	It("should create autoVirtualDC resources when the specified startSchedule and stopSchedule is blank", func() {
		avdc := &nyamberv1beta1.AutoVirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-avdc",
				Namespace: testNamespace,
			},
			Spec: nyamberv1beta1.AutoVirtualDCSpec{
				StartSchedule: "",
				StopSchedule:  "",
			},
		}
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should deny to update AutoVirtualDC resources", func() {
		avdc := makeAutoVirtualDC()
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())

		By("updating startSchedule")
		newAvdc := avdc.DeepCopy()
		newAvdc.Spec.StartSchedule = "0 2 * * *"
		err = k8sClient.Update(ctx, newAvdc)
		Expect(err).To(HaveOccurred())

		By("updating stopSchedule")
		newAvdc = avdc.DeepCopy()
		newAvdc.Spec.StopSchedule = "0 2 * * *"
		err = k8sClient.Update(ctx, newAvdc)
		Expect(err).To(HaveOccurred())

	})

	It("should be allowed to update timeoutDuration in AutoVirtualDC resources", func() {
		avdc := makeAutoVirtualDC()
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())

		By("updating timeoutDuration")
		avdc.Spec.TimeoutDuration = "0h"
		err = k8sClient.Update(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should be allowed to update template in AutoVirtualDC resources", func() {
		avdc := makeAutoVirtualDC()
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())
		By("updating vdc template")
		avdc.Spec.Template.Spec.NecoBranch = "hoge"
		err = k8sClient.Update(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should deny autoVirtualDC resources if the name of the autoVirtualDC conflicts with one of VirtualDC resources", func() {
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-avdc",
				Namespace: testNamespace,
			},
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(vdc), vdc)
		}).Should(Succeed())

		By("creating avdc in same namespace")
		avdc := makeAutoVirtualDC()
		err = k8sClient.Create(ctx, avdc)
		Expect(err).To(HaveOccurred())

		By("creating avdc in another namespace")
		avdc.Namespace = testAnotherNamespace
		err = k8sClient.Create(ctx, avdc)
		Expect(err).To(HaveOccurred())
	})

	It("should deny autoVirtualDC resources if the name of the autoVirtualDC conflicts with one of AutoVirtualDC resources", func() {
		avdc := makeAutoVirtualDC()
		err := k8sClient.Create(ctx, avdc)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(avdc), avdc)
		}).Should(Succeed())

		avdc = makeAutoVirtualDC()
		avdc.Namespace = testAnotherNamespace
		err = k8sClient.Create(ctx, avdc)
		Expect(err).To(HaveOccurred())
	})
})

func makeAutoVirtualDC() *nyamberv1beta1.AutoVirtualDC {
	return &nyamberv1beta1.AutoVirtualDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-avdc",
			Namespace: testNamespace,
		},
		Spec: nyamberv1beta1.AutoVirtualDCSpec{
			StartSchedule:   "0 1 * * *",
			StopSchedule:    "0 5 * * *",
			TimeoutDuration: "1h",
			Template: nyamberv1beta1.VirtualDC{
				Spec: nyamberv1beta1.VirtualDCSpec{
					NecoBranch:     "test",
					NecoAppsBranch: "test",
					SkipNecoApps:   false,
					Command:        []string{"test", "command"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				},
			},
		},
	}
}
