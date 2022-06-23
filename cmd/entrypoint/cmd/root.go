package cmd

import (
	"fmt"
	"net/http"

	"github.com/cybozu-go/nyamber/pkg/constants"
	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var listenAddr string
var log logr.Logger

type StatusResponse struct {
	Job []JobStatus `json:"jobs"`
}

type JobStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

var rootCmd = &cobra.Command{
	Use:          "entrypoint <JOB_NAME:COMMAND_PATH>...",
	Short:        "DC test pod entrypoint",
	Long:         "DC test pod entrypoint",
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		// run neco bootstrap
		// run neco-apps bootstrap
		// serve execution status of bootstrap process (status endpoint)
		mux := http.NewServeMux()
		mux.Handle("/"+constants.StatusEndPoint, http.HandlerFunc(statusHandler))
		serv := &well.HTTPServer{
			Server: &http.Server{
				Addr:    listenAddr,
				Handler: mux,
			},
		}
		log.Info("Entrypoint server start")
		if err := serv.ListenAndServe(); err != nil {
			return err
		}

		well.Stop()
		return well.Wait()
	},
}

func statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	fs := rootCmd.Flags()
	fs.StringVar(&listenAddr, "listen-address", fmt.Sprintf(":%d", constants.ListenPort), "Listening address and port.")
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	log = zapr.NewLogger(zapLog)
}
