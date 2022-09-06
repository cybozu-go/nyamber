package e2e

import (
	"encoding/json"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("Nyamber avdc e2e test", func() {
	It("should prepare", func() {
		By("applying valid avdc manifest")
		_, err := kubectl(nil, "apply", "-f", "./manifests/avdc_testcase.yaml")
		Expect(err).Should(Succeed())
	})

	It("should deny invalid avdc manifest", func() {
		_, err := kubectl(nil, "apply", "-f", "./manifests/avdc_testcase2.yaml")
		Expect(err).ShouldNot(Succeed())
	})

	It("should create vdc according to avdc", func() {
		By("checking vdc is created")
		Eventually(func() (*nyamberv1beta1.VirtualDC, error) {
			out, err := kubectl(nil, "get", "virtualdc", "auto-virtual-dc", "-o", "json")
			if err != nil {
				return nil, err
			}
			vdc := &nyamberv1beta1.VirtualDC{}
			err = json.Unmarshal(out, vdc)
			if err != nil {
				return nil, err
			}
			return vdc, nil
		}, 5).Should(
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Spec": MatchFields(IgnoreExtras, Fields{
					"NecoBranch":     Equal("release"),
					"NecoAppsBranch": Equal("release"),
					"Command":        Equal([]string{"env"}),
				}),
			})),
		)
	})

	It("should delete vdc and avdc", func() {
		_, err := kubectl(nil, "delete", "-f", "./manifests/avdc_testcase.yaml")
		Expect(err).Should(Succeed())

		Eventually(func() error {
			_, err := kubectl(nil, "get", "autovirtualdc", "auto-virtual-dc")
			return err
		}).Should(HaveOccurred())

		Eventually(func() error {
			_, err := kubectl(nil, "get", "virtualdc", "auto-virtual-dc")
			return err
		}).Should(HaveOccurred())
	})
})
