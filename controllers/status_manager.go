package controllers

import (
	"context"
	"errors"
	"sync"
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
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

func NewSecretUpdater(log logr.Logger, k8sClient client.Client) JobProcessManager {
	return &jobProcessManager{
		log:       log.WithName("SecretUpdater"),
		k8sClient: k8sClient,
		processes: map[string]*jobWatchProcess{},
	}
}

func (u *jobProcessManager) Start(vdc *nyamberv1beta1.VirtualDC) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.stopped {
		return errors.New("SecretUpdater is already stopped")
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
		vdc:          vdc,
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
			pod := &corev1.Pod{}
			if err := p.k8sClient.Get(ctx, client.ObjectKey{Namespace: p.vdcNamespace, Name: p.vdcName}, pod); err != nil {
				p.log.Error(err, "failed to get pod")
				continue
			}
		}

	}
}
