package cmd

import (
	"fmt"
	"net/http"

	"github.com/cybozu-go/nyamber/pkg/constants"
	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
)

var listenAddr string

var rootCmd = &cobra.Command{
	Use:   "entrypoint",
	Short: "DC test pod entrypoint",
	Long:  "DC test pod entrypoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		// run neco bootstrap
		// run neco-apps bootstrap
		// serve execution status of bootstrap process (status endpoint)
		mux := http.NewServeMux()
		mux.Handle("/"+constants.StatusEndPoint, http.HandlerFunc(statusHandler))
		serv := &well.HTTPServer{
			Server: &http.Server{
				Addr:    constants.ListenPort,
				Handler: mux,
			},
		}
		if err := serv.ListenAndServe(); err != nil {
			return err
		}

		env.Stop()
		return env.Wait()
		return nil
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
}
