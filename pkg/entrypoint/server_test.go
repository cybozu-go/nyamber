package entrypoint

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/cybozu-go/nyamber/pkg/constants"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

const apiAddr = "localhost:8080"

type testCase struct {
	name     string
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
	It("should state of commands are changed correctly", func() {
		testCases := []testCase{
			{
				name: "one successfull command",
				input: []Job{
					{
						Name:    "test1",
						Command: "sleep",
						Args:    []string{"3"},
					},
				},
				expected: []statusResponse{
					{Jobs: []job{{Name: "test1", Status: "Running"}}},
					{Jobs: []job{{Name: "test1", Status: "Completed"}}},
				},
			},
			{
				name: "one command which execute with exit code(1)",
				input: []Job{
					{
						Name:    "test1",
						Command: "false",
						Args:    []string{},
					}},
				expected: []statusResponse{
					{Jobs: []job{{Name: "test1", Status: "Failed"}}},
				},
			},
			{
				name: "one command which is not existed",
				input: []Job{
					{
						Name:    "test1",
						Command: "unknowncommand",
						Args:    []string{},
					}},
				expected: []statusResponse{
					{Jobs: []job{{Name: "test1", Status: "Failed"}}},
				},
			},
			{
				name: "one command which doesn't have permission",
				input: []Job{
					{
						Name:    "test1",
						Command: "./testresources/script_without_exec_permission.sh",
						Args:    []string{},
					}},
				expected: []statusResponse{
					{Jobs: []job{{Name: "test1", Status: "Failed"}}},
				},
			},
			{
				name: "two successful command",
				input: []Job{
					{
						Name:    "test1",
						Command: "sleep",
						Args:    []string{"5"},
					},
					{
						Name:    "test2",
						Command: "sleep",
						Args:    []string{"5"},
					}},
				expected: []statusResponse{
					{Jobs: []job{{Name: "test1", Status: "Running"}, {Name: "test2", Status: "Pending"}}},
					{Jobs: []job{{Name: "test1", Status: "Completed"}, {Name: "test2", Status: "Running"}}},
					{Jobs: []job{{Name: "test1", Status: "Completed"}, {Name: "test2", Status: "Completed"}}},
				},
			},
			{
				name: "first command is fail and second one is pending",
				input: []Job{
					{
						Name:    "test1",
						Command: "false",
						Args:    []string{},
					},
					{
						Name:    "test2",
						Command: "sleep",
						Args:    []string{"1"},
					}},
				expected: []statusResponse{
					{Jobs: []job{{Name: "test1", Status: "Failed"}, {Name: "test2", Status: "Pending"}}},
				},
			},
		}
		for _, tt := range testCases {
			By(tt.name)
			func() {
				cancel := startRunner(apiAddr, tt.input)
				defer cancel()
				for _, expected := range tt.expected {
					Eventually(getStatus, 10, 0.5).Should(Equal(&expected))
				}
			}()
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

func getStatus(g Gomega) *statusResponse {
	resp, err := http.Get(fmt.Sprintf("http://%s/%s", apiAddr, constants.StatusEndPoint))
	g.Expect(err).Should(Succeed())
	res := &statusResponse{}
	body, err := io.ReadAll(resp.Body)
	g.Expect(err).Should(Succeed())
	defer resp.Body.Close()
	err = json.Unmarshal(body, res)
	g.Expect(err).Should(Succeed())
	return res
}
