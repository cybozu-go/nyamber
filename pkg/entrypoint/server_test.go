package entrypoint

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

const apiAddr = "localhost:8080"

type testCase struct {
	input    []Job
	expected []statusResponse
}
type statusResponse struct {
	Jobs []job
}
type job struct {
	Name   string
	Status string
}

var log logr.Logger
var _ = BeforeSuite(func() {
	zapLog, _ := zap.NewDevelopment()
	log = zapr.NewLogger(zapLog)
})

var _ = Describe("entrypoint status API test", func() {
	It("should state of successful command changed from running to completed", func() {
		testcase := testCase{
			input: []Job{
				{
					Name:    "test1",
					Command: "sleep",
					Args:    []string{"1"},
				},
			},
			expected: []statusResponse{
				{Jobs: []job{{Name: "test1", Status: "Running"}}},
				{Jobs: []job{{Name: "test1", Status: "Completed"}}},
			},
		}

		cancel := startRunner(apiAddr, testcase.input)
		defer cancel()

		for _, expected := range testcase.expected {
			Eventually(gotStatus, 10, 0.5).Should(Equal(&expected))
		}
	})

	It("should state of failing command changed from running to failed", func() {
		testcase := testCase{
			input: []Job{
				{
					Name:    "test1",
					Command: "false",
					Args:    []string{},
				}},
			expected: []statusResponse{
				{Jobs: []job{{Name: "test1", Status: "Failed"}}},
			},
		}

		cancel := startRunner(apiAddr, testcase.input)
		defer cancel()

		for _, expected := range testcase.expected {
			Eventually(gotStatus, 10, 0.5).Should(Equal(&expected))
		}
	})

	It("should two sucessful command is executed in series", func() {

		testcase := testCase{
			input: []Job{
				{
					Name:    "test1",
					Command: "echo",
					Args:    []string{"1"},
				},
				{
					Name:    "test2",
					Command: "sleep",
					Args:    []string{"1"},
				}},
			expected: []statusResponse{
				{Jobs: []job{{Name: "test1", Status: "Completed"}, {Name: "test2", Status: "Running"}}},
				{Jobs: []job{{Name: "test1", Status: "Completed"}, {Name: "test2", Status: "Completed"}}},
			},
		}

		cancel := startRunner(apiAddr, testcase.input)
		defer cancel()

		for _, expected := range testcase.expected {
			Eventually(gotStatus, 10, 0.5).Should(Equal(&expected))
		}
	})
	It("should second command is pended when first command fails", func() {

		testcase := testCase{
			input: []Job{
				{
					Name:    "test1",
					Command: "false",
					Args:    []string{""},
				},
				{
					Name:    "test2",
					Command: "sleep",
					Args:    []string{"1"},
				}},
			expected: []statusResponse{
				{Jobs: []job{{Name: "test1", Status: "Failed"}, {Name: "test2", Status: "Pending"}}},
			},
		}

		cancel := startRunner(apiAddr, testcase.input)
		defer cancel()

		for _, expected := range testcase.expected {
			Eventually(gotStatus, 10, 0.5).Should(Equal(&expected))
		}
	})
})

func startRunner(listenAddr string, jobs []Job) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	runner := NewRunner(listenAddr, log, jobs)
	go func() {
		defer GinkgoRecover()
		Expect(runner.Run(ctx)).To(Succeed())
	}()
	return cancel
}

func gotStatus(g Gomega) (*statusResponse, error) {
	resp, err := http.Get("http://" + apiAddr + "/status")
	g.Expect(err).Should(Succeed())
	res := &statusResponse{}
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()
	json.Unmarshal(body, res)
	return res, nil
}
