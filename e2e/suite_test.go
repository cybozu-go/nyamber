package e2e

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	if !runE2E {
		t.Skip("no RUN_E2E environment variable")
	}
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultEventuallyPollingInterval(100 * time.Millisecond)
	RunSpecs(t, "E2e Suite")
}

var _ = BeforeSuite(func() {
	By("deleting all autovirtualDC and virtualDC")
	_, err := kubectl(nil, "delete", "autovirtualdc", "--all")
	Expect(err).NotTo(HaveOccurred())
	_, err = kubectl(nil, "delete", "virtualdc", "--all")
	Expect(err).NotTo(HaveOccurred())

	By("deploy namespace and configmap")
	_, err = kubectl(nil, "apply", "-f", "./manifests/namespace.yaml")
	Expect(err).Should(Succeed())
	_, err = kubectl(nil, "apply", "-k", "./manifests/script-config")
	Expect(err).Should(Succeed())
})
