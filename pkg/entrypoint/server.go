package entrypoint

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"github.com/cybozu-go/nyamber/pkg/constants"
	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
)

type JobState struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
}

const (
	JobStatusPending   = "Pending"
	JobStatusRunning   = "Running"
	JobStatusCompleted = "Completed"
	JobStatusFailed    = "Failed"
)

type Job struct {
	Name    string
	Command string
	Args    []string
}

type Runner struct {
	listenAddr string
	logger     logr.Logger
	jobs       []Job

	mutex     sync.Mutex
	jobStates []JobState
}

func NewRunner(listenAddr string, logger logr.Logger, jobs []Job) *Runner {
	runner := &Runner{
		listenAddr: listenAddr,
		logger:     logger,
		jobs:       jobs,
		jobStates:  make([]JobState, len(jobs)),
	}
	for i, job := range jobs {
		runner.jobStates[i].Name = job.Name
		runner.jobStates[i].Status = JobStatusPending
	}
	return runner
}

func (r *Runner) Run(ctx context.Context) error {
	env := well.NewEnvironment(ctx)
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
			Addr:    r.listenAddr,
			Handler: mux,
		},
	}
	r.logger.Info("Entrypoint server start")
	if err := serv.ListenAndServe(); err != nil {
		return err
	}

	env.Stop()
	return env.Wait()
}

func (r *Runner) runJobs(ctx context.Context) {
	for i, job := range r.jobs {
		r.logger.Info("execute job", "job_name", job.Name)
		e := exec.CommandContext(ctx, job.Command, job.Args...)
		startTime := time.Now().UTC().Format(time.RFC3339)
		r.mutex.Lock()
		r.jobStates[i].StartTime = startTime
		r.jobStates[i].Status = JobStatusRunning
		r.mutex.Unlock()

		err := e.Run()
		endTime := time.Now().UTC().Format(time.RFC3339)
		if err != nil {
			r.logger.Error(err, "job execution error", "job_name", job.Name)
			r.mutex.Lock()
			r.jobStates[i].EndTime = endTime
			r.jobStates[i].Status = JobStatusFailed
			r.mutex.Unlock()
			return
		}

		r.logger.Info("job completed", "job_name", job.Name)
		r.mutex.Lock()
		r.jobStates[i].EndTime = endTime
		r.jobStates[i].Status = JobStatusCompleted
		r.mutex.Unlock()
	}
}

func (r *Runner) statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	type StatusResponse struct {
		Jobs []JobState `json:"jobs"`
	}
	r.mutex.Lock()
	resp := &StatusResponse{Jobs: r.jobStates}
	data, err := json.Marshal(resp)
	r.mutex.Unlock()
	if err != nil {
		r.logger.Error(err, "Status handler")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(data)
}
