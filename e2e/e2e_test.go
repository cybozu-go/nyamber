package e2e

import (
	"encoding/json"
	"fmt"
	"regexp"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Nyamber", func() {
	It("should prepare", func() {
		vdcs := []string{"vdc_testcase", "vdc_testcase2", "vdc_testcase3", "avdc_testcase"}
		_, err := kubectl(nil, "apply", "-f", "./manifests/namespace.yaml")
		Expect(err).Should(Succeed())
		for _, v := range vdcs {
			By(v)
			_, err := kubectl(nil, "apply", "-f", fmt.Sprintf("./manifests/%s.yaml", v))
			Expect(err).Should(Succeed())
		}
	})

	It("should deny invalid avdc manifest", func() {
		_, err := kubectl(nil, "apply", "-f", "./manifests/avdc_testcase2.yaml")
		Expect(err).ShouldNot(Succeed())
	})

	It("should create resources", func() {
		testcases := []struct {
			name string
			args []string
			env  []corev1.EnvVar
		}{
			{
				"vdc-testcase",
				[]string{"neco_bootstrap:/neco-bootstrap"},
				[]corev1.EnvVar{
					{
						Name:      "NECO_BRANCH",
						Value:     "main",
						ValueFrom: nil,
					},
				},
			},
			{
				"vdc-testcase2",
				[]string{
					"neco_bootstrap:/neco-bootstrap",
					"neco_apps_bootstrap:/neco-apps-bootstrap",
					"user_defined_command:env",
				},
				[]corev1.EnvVar{
					{
						Name:      "NECO_BRANCH",
						Value:     "test",
						ValueFrom: nil,
					},
					{
						Name:      "NECO_APPS_BRANCH",
						Value:     "main",
						ValueFrom: nil,
					},
				},
			},
			{
				"vdc-testcase3",
				[]string{"neco_bootstrap:/neco-bootstrap", "neco_apps_bootstrap:/neco-apps-bootstrap", "user_defined_command:false"},
				[]corev1.EnvVar{
					{
						Name:      "NECO_BRANCH",
						Value:     "main",
						ValueFrom: nil,
					},
					{
						Name:      "NECO_APPS_BRANCH",
						Value:     "main",
						ValueFrom: nil,
					},
				},
			},
		}
		for _, tt := range testcases {
			By(tt.name)
			Eventually(func() (*corev1.Pod, error) {
				out, err := kubectl(nil, "get", "pod", "-n", "nyamber-pod", tt.name, "-o", "json")
				if err != nil {
					return nil, err
				}
				pod := &corev1.Pod{}
				err = json.Unmarshal(out, pod)
				if err != nil {
					return nil, err
				}
				return pod, nil
			}, 5).Should(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Spec": MatchFields(IgnoreExtras, Fields{
						"Containers": ContainElements(MatchFields(IgnoreExtras, Fields{
							"Args": Equal(tt.args),
							"Env":  Equal(tt.env),
						})),
					}),
				})),
			)
			Eventually(func() error {
				_, err := kubectl(nil, "get", "svc", "-n", "nyamber-pod", tt.name)
				if err != nil {
					return err
				}
				return nil
			}).Should(Succeed())
		}
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



	It("should execute all commands correctly", func() {
		testcases := []struct {
			name    string
			matcher types.GomegaMatcher
		}{
			{
				"vdc-testcase",
				ContainElements("+ echo neco-bootstrap"),
			},
			{
				"vdc-testcase2",
				ContainElements("+ echo neco-bootstrap", "+ echo neco-apps-bootstrap", "NECO_BRANCH=test", "NECO_APPS_BRANCH=main"),
			},
			{
				"vdc-testcase3",
				ContainElements("+ echo neco-bootstrap", "+ echo neco-apps-bootstrap", ContainSubstring("job execution error")),
			},
		}
		for _, tt := range testcases {
			By(tt.name)
			Eventually(func() ([]string, error) {
				out, err := kubectl(nil, "logs", "-n", "nyamber-pod", tt.name)
				if err != nil {
					return nil, err
				}
				return regexp.MustCompile("\r\n|\n").Split(string(out), -1), nil
			}, 3).Should(tt.matcher)
		}
	})

	It("should update status of vdc resource", func() {
		testcases := []struct {
			name      string
			condition Fields
		}{
			{
				"vdc-testcase",
				Fields{
					"Reason": Equal(nyamberv1beta1.ReasonOK),
					"Type":   Equal(nyamberv1beta1.TypePodJobCompleted),
					"Status": Equal(metav1.ConditionTrue),
				},
			},
			{
				"vdc-testcase2",
				Fields{
					"Reason": Equal(nyamberv1beta1.ReasonOK),
					"Type":   Equal(nyamberv1beta1.TypePodJobCompleted),
					"Status": Equal(metav1.ConditionTrue),
				},
			},
			{
				"vdc-testcase3",
				Fields{
					"Reason": Equal(nyamberv1beta1.ReasonServiceCreatedFailed),
					"Type":   Equal(nyamberv1beta1.TypePodJobCompleted),
					"Status": Equal(metav1.ConditionFalse),
				},
			},
		}
		for _, tt := range testcases {
			By(tt.name)
			Eventually(func() ([]metav1.Condition, error) {
				out, err := kubectl(nil, "get", "vdc", tt.name, "-o", "json")
				if err != nil {
					return nil, err
				}
				vdc := &nyamberv1beta1.VirtualDC{}
				if err := json.Unmarshal(out, vdc); err != nil {
					return nil, err
				}
				return vdc.Status.Conditions, nil
			}, 10).Should(ContainElements(MatchFields(IgnoreExtras, tt.condition)))
		}
	})

	It("should not modify the existed vdc resources", func() {
		_, err := kubectl(nil, "apply", "-f", "./manifests/vdc_withsamename.yaml")
		Expect(err).Should(HaveOccurred())
	})

	It("should not deploy vdc resources if the vdc resources with same name exists", func() {
		_, err := kubectl(nil, "apply", "-f", "./manifests/vdc_withsamename.yaml", "-n", "nyamber-test")
		Expect(err).Should(HaveOccurred())
	})

	It("should delete pod and svc when vdc resource is deleted", func() {
		vdcs := []string{"vdc_testcase", "vdc_testcase2", "vdc_testcase3"}
		for _, v := range vdcs {
			_, err := kubectl(nil, "delete", "-f", fmt.Sprintf("./manifests/%s.yaml", v))
			Expect(err).Should(Succeed())
		}
		for _, v := range vdcs {
			By(v)
			Eventually(func() error {
				_, err := kubectl(nil, "get", "pod", "-n", "nyamber-pod", v)
				if err != nil {
					return err
				}
				return nil
			}).Should(HaveOccurred())
			Eventually(func() error {
				_, err := kubectl(nil, "get", "svc", "-n", "nyamber-pod", v)
				if err != nil {
					return err
				}
				return nil
			}).Should(HaveOccurred())
		}
	})
	
	It("should delete vdc and avdc", func() {
		_, err := kubectl(nil, "delete", "-f", "./manifests/avdc_testcase.yaml")
		Expect(err).Should(Succeed())


		Eventually(func() error {
			_, err := kubectl(nil, "get", "autovirtualdc")
			if err != nil {
				return err
			}
			return nil
		}).Should(HaveOccurred())
		Eventually(func() error {
			_, err := kubectl(nil, "get", "virtualdc")
			if err != nil {
				return err
			}
			return nil
		}).Should(HaveOccurred())

	})

})
