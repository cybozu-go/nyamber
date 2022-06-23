package entrypoint

import (
	"net/http"

	"github.com/cybozu-go/nyamber/pkg/constants"
	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
)

type StatusResponse struct {
	Job []JobStatus `json:"jobs"`
}

type JobStatus struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

type Job struct {
	Name    string
	Command string
}

type Runner struct {
	ListenAddr string
	Logger     logr.Logger
	Jobs       []Job
}

func (r *Runner) Run() error {
	mux := http.NewServeMux()
	mux.Handle("/"+constants.StatusEndPoint, http.HandlerFunc(r.statusHandler))
	serv := &well.HTTPServer{
		Server: &http.Server{
			Addr:    r.ListenAddr,
			Handler: mux,
		},
	}
	r.Logger.Info("Entrypoint server start")
	if err := serv.ListenAndServe(); err != nil {
		return err
	}

	well.Stop()
	return well.Wait()
}

func (r *Runner) statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}
