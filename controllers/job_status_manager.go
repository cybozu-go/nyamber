package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	"github.com/cybozu-go/nyamber/pkg/entrypoint"
	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const interval time.Duration = time.Second * 10 // for development.

// requestTimeout bounds each status request so that a stopped process does not wait for an unresponsive pod.
const requestTimeout time.Duration = time.Second * 5

type JobProcessManager interface {
	Start(vdc *nyamberv1beta1.VirtualDC) error
	Stop(vdc *nyamberv1beta1.VirtualDC) error
	StopAll()
}

type jobProcessManager struct {
	log          logr.Logger
	k8sClient    client.Client
	mu           sync.Mutex
	stopped      bool
	processes    map[string]*jobWatchProcess
	podNamespace string
}

func NewJobProcessManager(log logr.Logger, k8sClient client.Client, podNamespace string) JobProcessManager {
	return &jobProcessManager{
		log:          log.WithName("JobProcessManager"),
		k8sClient:    k8sClient,
		processes:    map[string]*jobWatchProcess{},
		podNamespace: podNamespace,
	}
}

func (j *jobProcessManager) Start(vdc *nyamberv1beta1.VirtualDC) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.stopped {
		return errors.New("JobProcessManager is already stopped")
	}

	vdcNamespacedName := types.NamespacedName{Namespace: vdc.Namespace, Name: vdc.Name}.String()
	if _, ok := j.processes[vdcNamespacedName]; !ok {
		process := newJobWatchProcess(
			j.log.WithValues("jobWatchProcess", vdcNamespacedName),
			j.k8sClient,
			vdc,
			j.podNamespace,
		)
		process.start()
		j.processes[vdcNamespacedName] = process
	}
	return nil
}

func (j *jobProcessManager) Stop(vdc *nyamberv1beta1.VirtualDC) error {
	j.mu.Lock()

	vdcNamespacedName := types.NamespacedName{Namespace: vdc.Namespace, Name: vdc.Name}.String()
	process, ok := j.processes[vdcNamespacedName]
	if !ok {
		j.mu.Unlock()
		return nil
	}
	delete(j.processes, vdcNamespacedName)
	j.mu.Unlock()

	// Wait for the process without holding the lock so that Start for other VirtualDCs is not blocked.
	return process.stop()
}

func (j *jobProcessManager) StopAll() {
	j.mu.Lock()
	defer j.mu.Unlock()

	for _, process := range j.processes {
		process.stop()
	}
	j.processes = nil
	j.stopped = true
}

type jobWatchProcess struct {
	// Given from outside. Not update internally.
	log          logr.Logger
	k8sClient    client.Client
	vdcNamespace string
	vdcName      string
	podNamespace string
	httpClient   *http.Client
	cancel       func()
	env          *well.Environment
}

func newJobWatchProcess(log logr.Logger, k8sClient client.Client, vdc *nyamberv1beta1.VirtualDC, podNamespace string) *jobWatchProcess {
	return &jobWatchProcess{
		log:          log,
		k8sClient:    k8sClient,
		vdcNamespace: vdc.Namespace,
		vdcName:      vdc.Name,
		podNamespace: podNamespace,
		httpClient:   &http.Client{Timeout: requestTimeout},
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
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for i := 0; i < 3; i++ {
				retry, err := p.updateStatus(ctx)
				if err != nil {
					p.log.Error(err, "failed to update status")
				}
				if retry {
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second * 1):
					}
					continue
				}
				break
			}
		}
	}
}

func (p *jobWatchProcess) updateStatus(ctx context.Context) (bool, error) {
	beforeVdc := &nyamberv1beta1.VirtualDC{}
	if err := p.k8sClient.Get(ctx, client.ObjectKey{Name: p.vdcName, Namespace: p.vdcNamespace}, beforeVdc); err != nil {
		return false, err
	}
	state, err := p.getPodState(ctx)
	if err != nil {
		return false, err
	}
	vdc := beforeVdc.DeepCopy()
	if state == podStateActive {
		jobStates, err := p.getJobStates(ctx)
		if err != nil {
			return false, err
		}
		for _, job := range jobStates.Jobs {
			meta.SetStatusCondition(&vdc.Status.Conditions, getJobCondition(job))
			if job.Status != entrypoint.JobStatusCompleted {
				break
			}
		}
	} else if meta.IsStatusConditionTrue(vdc.Status.Conditions, nyamberv1beta1.TypePodCreated) {
		// The controller does not recreate a pod that has disappeared or failed (e.g. evicted on a node reboot).
		// Treat it as a job failure so that AutoVirtualDC recreates the VirtualDC.
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:    nyamberv1beta1.TypePodJobCompleted,
			Status:  metav1.ConditionFalse,
			Reason:  nyamberv1beta1.ReasonPodJobCompletedFailed,
			Message: state.String(),
		})
	}
	if !equality.Semantic.DeepEqual(vdc.Status, beforeVdc.Status) {
		p.log.Info("update status", "status", vdc.Status, "before", beforeVdc.Status)
		if err := p.k8sClient.Status().Update(ctx, vdc); err != nil {
			return apierrors.IsConflict(err), err
		}
	}
	return false, nil
}

// podState represents whether the runner pod can run the jobs.
type podState int

const (
	podStateActive podState = iota
	podStateNotFound
	podStateFailed
)

func (s podState) String() string {
	switch s {
	case podStateActive:
		return "pod is active"
	case podStateNotFound:
		return "pod does not exist"
	case podStateFailed:
		return "pod is in Failed phase"
	}
	return "unknown pod state"
}

func (p *jobWatchProcess) getPodState(ctx context.Context) (podState, error) {
	pod := &corev1.Pod{}
	if err := p.k8sClient.Get(ctx, client.ObjectKey{Namespace: p.podNamespace, Name: p.vdcName}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return podStateNotFound, nil
		}
		return podStateActive, err
	}
	if pod.Status.Phase == corev1.PodFailed {
		return podStateFailed, nil
	}
	return podStateActive, nil
}

func (p *jobWatchProcess) getJobStates(ctx context.Context) (*entrypoint.StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s.%s/%s", p.vdcName, p.podNamespace, constants.StatusEndPoint), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
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
		cond.Message = job.Name
	case entrypoint.JobStatusRunning:
		cond.Status = metav1.ConditionFalse
		cond.Reason = nyamberv1beta1.ReasonPodJobCompletedRunning
		cond.Message = job.Name
	case entrypoint.JobStatusPending:
		cond.Status = metav1.ConditionFalse
		cond.Reason = nyamberv1beta1.ReasonPodJobCompletedPending
		cond.Message = job.Name
	case entrypoint.JobStatusCompleted:
		cond.Status = metav1.ConditionTrue
		cond.Reason = nyamberv1beta1.ReasonOK
	}
	return cond
}
