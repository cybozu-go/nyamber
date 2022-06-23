package entrypoint

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"time"

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
	Args    []string
}

type Runner struct {
	ListenAddr string
	Logger     logr.Logger
	Jobs       []Job
}

func (r *Runner) Run(ctx context.Context) error {
	env := well.NewEnvironment(ctx)
	// env.Go(r.runJobs)
	// env.Stop()
	// defer env.Cancel(errors.New("Job canceled"))
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		r.runJobs(cctx)
	}()

	mux := http.NewServeMux()
	mux.Handle("/"+constants.StatusEndPoint, http.HandlerFunc(r.statusHandler))
	serv := &well.HTTPServer{
		Env: env,
		Server: &http.Server{
			Addr:    r.ListenAddr,
			Handler: mux,
		},
	}
	r.Logger.Info("Entrypoint server start")
	if err := serv.ListenAndServe(); err != nil {
		return err
	}

	env.Stop()
	return env.Wait()
}

func (r *Runner) runJobs(ctx context.Context) {
	for _, job := range r.Jobs {
		r.Logger.Info("execute job", "job_name", job.Name)
		e := exec.Command(job.Command, job.Args...)
		startTime := time.Now()
		err := e.Run()
		endTime := time.Now()
		fmt.Printf("start=%s end=%s\n", startTime, endTime)
		if err != nil {
			r.Logger.Error(err, "job execution error")
			return
		}
	}
}

func (r *Runner) statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}
