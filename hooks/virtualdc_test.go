package hooks

import (
	"context"
	"errors"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testVdcNamespace        string = "test-vdc-ns"
	testAnotherVdcNamespace string = "another-vdc-ns"
)

var _ = Describe("VirtualDC validator", func() {
	ctx := context.Background()

	BeforeEach(func() {
	})

	AfterEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &nyamberv1beta1.VirtualDC{}, client.InNamespace(testVdcNamespace))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &nyamberv1beta1.VirtualDC{}, client.InNamespace(testAnotherVdcNamespace))
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() error {
			vdcs := &nyamberv1beta1.VirtualDCList{}
			if err := k8sClient.List(ctx, vdcs); err != nil {
				return err
			}
			if len(vdcs.Items) != 0 {
				return errors.New("vdcs is not deleted")
			}
			return nil
		}).Should(Succeed())
		time.Sleep(100 * time.Millisecond)
	})

	It("should allow to create virtualdc resources", func() {
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
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
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())
	})

	It("should deny to create virtualdc resources when a resource with same name already exists", func() {
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
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
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("creating another virtualdc resource with same name")
		anotherVdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testAnotherVdcNamespace,
			},
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
		}
		err = k8sClient.Create(ctx, anotherVdc)
		Expect(err).To(HaveOccurred())
	})

	It("should deny to update virtualdc resources", func() {
		vdc := &nyamberv1beta1.VirtualDC{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-vdc",
				Namespace: testVdcNamespace,
			},
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
		}
		err := k8sClient.Create(ctx, vdc)
		Expect(err).NotTo(HaveOccurred())

		By("updating NecoBranch value")
		newVdc := vdc.DeepCopy()
		newVdc.Spec.NecoBranch = "modified"
		err = k8sClient.Update(ctx, newVdc)
		Expect(err).To(HaveOccurred())

		By("updating NecoAppsBranch value")
		newVdc = vdc.DeepCopy()
		newVdc.Spec.NecoAppsBranch = "apps-modified"
		err = k8sClient.Update(ctx, newVdc)
		Expect(err).To(HaveOccurred())

		By("updating Command")
		newVdc = vdc.DeepCopy()
		newVdc.Spec.Command = []string{"modified"}
		err = k8sClient.Update(ctx, newVdc)
		Expect(err).To(HaveOccurred())

		By("updating SkipNecoApps")
		newVdc = vdc.DeepCopy()
		newVdc.Spec.SkipNecoApps = true
		err = k8sClient.Update(ctx, newVdc)
		Expect(err).To(HaveOccurred())

		By("updating Resources")
		newVdc = vdc.DeepCopy()
		newVdc.Spec.Resources.Limits[corev1.ResourceCPU] = resource.MustParse("200m")
		err = k8sClient.Update(ctx, newVdc)
		Expect(err).To(HaveOccurred())
	})
})
