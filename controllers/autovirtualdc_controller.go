/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	cron "github.com/robfig/cron/v3"
)

// AutoVirtualDCReconciler reconciles a AutoVirtualDC object
type AutoVirtualDCReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Clock
}

type Clock interface {
	Now() time.Time
}

//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=autovirtualdcs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=autovirtualdcs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=autovirtualdcs/finalizers,verbs=update
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AutoVirtualDC object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *AutoVirtualDCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	avdc := &nyamberv1beta1.AutoVirtualDC{}
	if err := r.Get(ctx, req.NamespacedName, avdc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !avdc.ObjectMeta.DeletionTimestamp.IsZero() {
		finalizeResult, err := r.finalize(ctx, avdc)
		if err != nil {
			logger.Error(err, "Finalize error")
			return ctrl.Result{}, err
		}
		return finalizeResult, nil
	}

	if !controllerutil.ContainsFinalizer(avdc, constants.FinalizerName) {
		controllerutil.AddFinalizer(avdc, constants.FinalizerName)
		err := r.Update(ctx, avdc)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	defer func(before nyamberv1beta1.AutoVirtualDCStatus) {
		if equality.Semantic.DeepEqual(avdc.Status, before) {
			return
		}

		logger.Info("update status", "status", avdc.Status, "before", before)
		if err2 := r.Status().Update(ctx, avdc); err2 != nil {
			logger.Error(err2, "failed to update status")
			err = err2
		}
	}(*avdc.Status.DeepCopy())

	if avdc.Spec.StartSchedule == "" && avdc.Spec.StopSchedule == "" {
		requeue, err := r.reconcileVirtualDC(ctx, avdc)
		if err != nil {
			logger.Error(err, "failed to reconcile VirtualDC")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: requeue}, nil
	}

	if avdc.Status.NextStartTime == nil || avdc.Status.NextStopTime == nil {
		if err := r.updateStatusTime(ctx, avdc); err != nil {
			logger.Error(err, "failed to update avdc status")
			return ctrl.Result{}, err
		}
		if avdc.Status.NextStopTime.Before(avdc.Status.NextStartTime){
			nextStartTime := metav1.NewTime(r.Now())
			avdc.Status.NextStartTime = &nextStartTime
		}

		return ctrl.Result{Requeue: true}, nil
	}

	now := r.Now()
	// case 1
	if now.Before(avdc.Status.NextStartTime.Time) && now.Before(avdc.Status.NextStopTime.Time) {
		if avdc.Status.NextStartTime.Before(avdc.Status.NextStopTime) {
			return ctrl.Result{RequeueAfter: avdc.Status.NextStartTime.Sub(now)}, nil
		}
		return ctrl.Result{RequeueAfter: avdc.Status.NextStopTime.Sub(now)}, nil
	}

	// case 2
	if (avdc.Status.NextStartTime.Time.Before(now) || avdc.Status.NextStartTime.Time.Equal(now)) && now.Before(avdc.Status.NextStopTime.Time) {
		requeue, err := r.reconcileVirtualDC(ctx, avdc)
		if err != nil {
			logger.Error(err, "failed to reconcile VirtualDC")
			return ctrl.Result{}, err
		}
		if requeue {
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.updateStatusTime(ctx, avdc); err != nil {
			logger.Error(err, "failed to update avdc status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// case 3,4,5
	vdc := &nyamberv1beta1.VirtualDC{}
	vdc.Name = avdc.Name
	vdc.Namespace = avdc.Namespace
	err = r.Delete(ctx, vdc)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to delete vdc")
		return ctrl.Result{}, err
	}
	if apierrors.IsNotFound(err) {
		logger.Info("vdc is already deleted")
	} else {
		logger.Info("vdc is deleted successfully")
	}
	if err := r.updateStatusTime(ctx, avdc); err != nil {
		logger.Error(err, "failed to update avdc status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AutoVirtualDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nyamberv1beta1.AutoVirtualDC{}).
		Owns(&nyamberv1beta1.VirtualDC{}).
		Complete(r)
}

func (r *AutoVirtualDCReconciler) createVirtualDC(ctx context.Context, avdc *nyamberv1beta1.AutoVirtualDC) error {
	logger := log.FromContext(ctx)

	vdc := &nyamberv1beta1.VirtualDC{}

	err := r.Get(ctx, client.ObjectKey{Namespace: avdc.Namespace, Name: avdc.Name}, vdc)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}

	vdc = &nyamberv1beta1.VirtualDC{
		ObjectMeta: metav1.ObjectMeta{
			Name:      avdc.Name,
			Namespace: avdc.Namespace,
		},
		Spec: avdc.Spec.Template.Spec,
	}

	err = ctrl.SetControllerReference(avdc, vdc, r.Scheme)
	if err != nil {
		return err
	}

	err = r.Create(ctx, vdc)
	if err != nil {
		return err
	}

	logger.Info("VirtualDC created")
	return nil
}

func (r *AutoVirtualDCReconciler) finalize(ctx context.Context, avdc *nyamberv1beta1.AutoVirtualDC) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("finalize start")
	if !controllerutil.ContainsFinalizer(avdc, constants.FinalizerName) {
		return ctrl.Result{}, nil
	}

	controllerutil.RemoveFinalizer(avdc, constants.FinalizerName)
	err := r.Update(ctx, avdc)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("Finalize succeeded")
	return ctrl.Result{}, nil
}

func (r *AutoVirtualDCReconciler) updateStatusTime(ctx context.Context, avdc *nyamberv1beta1.AutoVirtualDC) error {
	specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	startSched, err := specParser.Parse(avdc.Spec.StartSchedule)
	if err != nil {
		return err
	}
	stopSched, err := specParser.Parse(avdc.Spec.StopSchedule)
	if err != nil {
		return err
	}

	nextStartTime := metav1.NewTime(startSched.Next(r.Now()))
	avdc.Status.NextStartTime = &nextStartTime
	nextStopTime := metav1.NewTime(stopSched.Next(r.Now()))
	avdc.Status.NextStopTime = &nextStopTime

	return nil
}

// reconcileVirtualDC reconciles VirtualDC and returns whether to requeue or not and error.
func (r *AutoVirtualDCReconciler) reconcileVirtualDC(ctx context.Context, avdc *nyamberv1beta1.AutoVirtualDC) (bool, error) {
	logger := log.FromContext(ctx)

	vdc := &nyamberv1beta1.VirtualDC{}
	err := r.Get(ctx, client.ObjectKeyFromObject(avdc), vdc)
	if err != nil && !apierrors.IsNotFound(err) {
		logger.Error(err, "failed to get vdc")
		return false, err
	}
	if apierrors.IsNotFound(err) {
		if err := r.createVirtualDC(ctx, avdc); err != nil {
			logger.Error(err, "failed to create vdc")
			return false, err
		}
		return true, nil
	}

	if !meta.IsStatusConditionTrue(vdc.Status.Conditions, nyamberv1beta1.TypePodJobCompleted) {
		return true, nil
	}

	return false, nil
}

type RealClock struct{}

func (r *RealClock) Now() time.Time {
	return time.Now()
}
