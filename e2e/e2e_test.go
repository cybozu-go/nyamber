package e2e

import (
	_ "embed"
	"encoding/json"
	"fmt"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Nyamber", func() {
	It("should prepare", func() {
		Eventually(func() error {
			_, err := kubectl(nil, "apply", "-f", "../config/dev/namespaces.yaml")
			return err
		}).Should(Succeed())
		Eventually(func() error {
			_, err := kubectl(nil, "apply", "-f", "./manifests/vdc_sample.yaml")
			return err
		}).Should(Succeed())
	})

	It("should create resources", func() {
		Eventually(func() error {
			_, err := kubectl(nil, "get", "pod", "-n", "nyamber-pod", "virtualdc-sample")
			if err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
		Eventually(func() error {
			_, err := kubectl(nil, "get", "svc", "-n", "nyamber-pod", "virtualdc-sample")
			if err != nil {
				return err
			}
			return nil
		}).Should(Succeed())
	})

	It("should update status in vdc resource", func() {
		Eventually(func() ([]metav1.Condition, error) {
			out, err := kubectl(nil, "get", "vdc", "virtualdc-sample", "-o", "json")
			if err != nil {
				return nil, err
			}
			vdc := &nyamberv1beta1.VirtualDC{}
			if err := json.Unmarshal(out, vdc); err != nil {
				return nil, err
			}
			return vdc.Status.Conditions, nil
		}, 10).Should(
			gstruct.MatchElements(func(element interface{}) string {
				return fmt.Sprintf("%v", element.(metav1.Condition).Type)
			},
				gstruct.IgnoreExtras,
				gstruct.Elements{
					nyamberv1beta1.TypePodJobCompleted: gstruct.MatchFields(gstruct.IgnoreExtras,
						gstruct.Fields{
							"Type":   Equal(nyamberv1beta1.TypePodJobCompleted),
							"Reason": Equal(nyamberv1beta1.ReasonOK),
						}),
				}),
		)
	})

	It("should not modify the existed vdc resources", func() {
		Eventually(func() error {
			_, err := kubectl(nil, "apply", "-f", "./manifests/vdc_sample2.yaml")
			return err
		}).Should(HaveOccurred())
	})
})
