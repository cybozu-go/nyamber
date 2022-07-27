package e2e

import (
	_ "embed"
	"encoding/json"
	"fmt"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
		Eventually(func() error {
			out, err := kubectl(nil, "get", "vdc", "virtualdc-sample", "-o", "json")
			if err != nil {
				return err
			}
			vdc := &nyamberv1beta1.VirtualDC{}
			if err := json.Unmarshal(out, vdc); err != nil {
				return err
			}
			for _, cond := range vdc.Status.Conditions {
				if cond.Type == nyamberv1beta1.TypePodJobCompleted && cond.Reason == nyamberv1beta1.ReasonOK {
					return nil
				}
			}
			return fmt.Errorf("Job is not completed")
		}).Should(Succeed(), 10)
	})
})
