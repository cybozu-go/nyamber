package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	"github.com/cybozu-go/nyamber/pkg/entrypoint"
	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const interval time.Duration = time.Second * 1 // for development.

type JobProcessManager interface {
	Start(vdc *nyamberv1beta1.VirtualDC) error
	Stop(vdc *nyamberv1beta1.VirtualDC) error
	StopAll()
}

type jobProcessManager struct {
	log       logr.Logger
	k8sClient client.Client
	mu        sync.Mutex
	stopped   bool
	processes map[string]*jobWatchProcess
}

func NewJobProcessManager(log logr.Logger, k8sClient client.Client) JobProcessManager {
	return &jobProcessManager{
		log:       log.WithName("JobProcessManager"),
		k8sClient: k8sClient,
		processes: map[string]*jobWatchProcess{},
	}
}

func (u *jobProcessManager) Start(vdc *nyamberv1beta1.VirtualDC) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.stopped {
		return errors.New("JobProcessManager is already stopped")
	}

	vdcNamespacedName := types.NamespacedName{Namespace: vdc.Namespace, Name: vdc.Name}.String()
	if _, ok := u.processes[vdcNamespacedName]; !ok {
		process := newJobWatchProcess(
			u.log.WithValues("jobWatchProcess", vdcNamespacedName),
			u.k8sClient,
			vdc,
		)
		process.start()
		u.processes[vdcNamespacedName] = process
	}
	return nil
}

func (u *jobProcessManager) Stop(vdc *nyamberv1beta1.VirtualDC) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	vdcNamespacedName := types.NamespacedName{Namespace: vdc.Namespace, Name: vdc.Name}.String()
	if process, ok := u.processes[vdcNamespacedName]; ok {
		if err := process.stop(); err != nil {
			return err
		}
		delete(u.processes, vdcNamespacedName)
	}
	return nil
}

func (u *jobProcessManager) StopAll() {
	u.mu.Lock()
	defer u.mu.Unlock()

	for _, process := range u.processes {
		process.stop()
	}
	u.processes = nil
	u.stopped = true
}

type jobWatchProcess struct {
	// Given from outside. Not update internally.
	log          logr.Logger
	k8sClient    client.Client
	vdcNamespace string
	vdcName      string
	vdc          *nyamberv1beta1.VirtualDC
	cancel       func()
	env          *well.Environment
}

func newJobWatchProcess(log logr.Logger, k8sClient client.Client, vdc *nyamberv1beta1.VirtualDC) *jobWatchProcess {
	return &jobWatchProcess{
		log:          log,
		k8sClient:    k8sClient,
		vdcNamespace: vdc.Namespace,
		vdcName:      vdc.Name,
	}
}

func (p *jobWatchProcess) start() {
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.env = well.NewEnvironment(ctx)
	p.env.Go(func(ctx context.Context) error {
		p.run(ctx)
		return nil
	})
	p.env.Stop()
}

func (p *jobWatchProcess) stop() error {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
		if err := p.env.Wait(); err != nil {
			return err
		}
	}
	return nil
}

func (p *jobWatchProcess) run(ctx context.Context) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		UPDATE_STATUS:
			beforeVdc := &nyamberv1beta1.VirtualDC{}
			if err := p.k8sClient.Get(ctx, client.ObjectKey{Name: p.vdcName, Namespace: p.vdcNamespace}, beforeVdc); err != nil {
				p.log.Error(err, "failed to get vdc")
				continue
			}
			jobStates, err := p.getJobStates()
			if err != nil {
				p.log.Error(err, "failed to get job states")
				continue
			}
			vdc := beforeVdc.DeepCopy()
			// Completed Running -> Running
			// Failed Pending -> Pending
			// TODO: implement merge logic
			for _, job := range jobStates.Jobs {
				switch job.Status {
				case entrypoint.JobStatusFailed:
					// failed
					meta.SetStatusCondition(&vdc.Status.Conditions, getJobCondition(job))
					break
				case entrypoint.JobStatusRunning:
					// running
					meta.SetStatusCondition(&vdc.Status.Conditions, getJobCondition(job))
					break
				case entrypoint.JobStatusPending:
					// pending
					meta.SetStatusCondition(&vdc.Status.Conditions, getJobCondition(job))
					break
				default:
					// completed
					continue
				}
				meta.SetStatusCondition(&vdc.Status.Conditions, getJobCondition(job))
			}
			if !equality.Semantic.DeepEqual(vdc.Status, beforeVdc.Status) {
				p.log.Info("update status", "status", vdc.Status, "before", beforeVdc.Status)
				if err := p.k8sClient.Status().Update(ctx, vdc); err != nil {
					p.log.Error(err, "failed to update status")
					if apierrors.IsConflict(err) {
						goto UPDATE_STATUS
					}
				}
			}
		}

	}
}

func (p *jobWatchProcess) getJobStates() (*entrypoint.StatusResponse, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s", p.vdcName, constants.StatusEndPoint))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data := make([]byte, 0, 256)
	if _, err := resp.Body.Read(data); err != nil {
		return nil, err
	}
	statusResp := &entrypoint.StatusResponse{}
	if err := json.Unmarshal(data, statusResp); err != nil {
		return nil, err
	}
	return statusResp, nil
}

func getJobCondition(job entrypoint.JobState) metav1.Condition {
	cond := metav1.Condition{
		Type: nyamberv1beta1.TypePodJobCompleted,
	}
	switch job.Status {
	case entrypoint.JobStatusFailed:
		cond.Status = metav1.ConditionFalse
		cond.Reason = nyamberv1beta1.ReasonPodJobCompletedFailed
		cond.Message = fmt.Sprintf("%s is failed", job.Name)
	case entrypoint.JobStatusRunning:
		cond.Status = metav1.ConditionFalse
		cond.Reason = nyamberv1beta1.ReasonPodJobCompletedRunning
		cond.Message = fmt.Sprintf("%s is running", job.Name)
	case entrypoint.JobStatusPending:
		cond.Status = metav1.ConditionFalse
		cond.Reason = nyamberv1beta1.ReasonPodJobCompletedPending
		cond.Message = fmt.Sprintf("%s is pending", job.Name)
	case entrypoint.JobStatusCompleted:
		cond.Status = metav1.ConditionTrue
		cond.Reason = nyamberv1beta1.ReasonOK
	}
	return cond
}
