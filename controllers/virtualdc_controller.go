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

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// VirtualDCReconciler reconciles a VirtualDC object
type VirtualDCReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	PodNameSpace string
}

//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VirtualDC object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *VirtualDCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	vdc := &nyamberv1beta1.VirtualDC{}
	if err := r.Get(ctx, req.NamespacedName, vdc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if vdc.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(vdc, constants.FinalizerName) {
			controllerutil.AddFinalizer(vdc, constants.FinalizerName)
			err = r.Update(ctx, vdc)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if err := r.finalize(ctx, vdc); err != nil {
			logger.Error(err, "Finalize error")
			return ctrl.Result{}, err
		}
		logger.Info("Finalize succeeded")
		return ctrl.Result{}, nil
	}

	defer func(before nyamberv1beta1.VirtualDCStatus) {
		if !equality.Semantic.DeepEqual(vdc.Status, before) {
			logger.Info("update status", "status", vdc.Status, "before", before)
			if err2 := r.Status().Update(ctx, vdc); err2 != nil {
				logger.Error(err2, "failed to update status")
				err = err2
			}
		}
	}(*vdc.Status.DeepCopy())

	if !meta.IsStatusConditionTrue(vdc.Status.Conditions, nyamberv1beta1.TypePodCreated) {
		if err := r.createPod(ctx, vdc); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.updateStatus(ctx, vdc); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("reconcile succeeded")
	return ctrl.Result{}, nil
}

func (r *VirtualDCReconciler) createPod(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	logger := log.FromContext(ctx)
	// get pod templates from configMap

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vdc.Name,
			Namespace: r.PodNameSpace,
			Labels:    map[string]string{constants.OwnerNamespace: vdc.GetNamespace()},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "virtualdc-pod",
					Image: "quay.io/cybozu/testhttpd:0",
				},
			},
		},
	}

	if err := r.Create(ctx, pod); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.TypePodCreated,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.ReasonPodCreatedFailed,
				Message: err.Error(),
			})
			return err
		}
		if err := r.Get(ctx, client.ObjectKey{Namespace: r.PodNameSpace, Name: vdc.Name}, pod); err != nil {
			return err
		}
		owner := pod.Labels[constants.OwnerNamespace]
		if owner != vdc.Namespace {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.TypePodCreated,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.ReasonPodCreatedConflict,
				Message: "Resource with same name already exists in another namespace",
			})
			return err
		}
		logger.Info("Pod already exists")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodCreated,
			Status: metav1.ConditionTrue,
			Reason: nyamberv1beta1.ReasonOK,
		})
		return nil
	}
	logger.Info("Pod created")
	meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
		Type:   nyamberv1beta1.TypePodCreated,
		Status: metav1.ConditionTrue,
		Reason: nyamberv1beta1.ReasonOK,
	})
	return nil
}

func (r *VirtualDCReconciler) updateStatus(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.PodNameSpace, Name: vdc.Name}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.TypePodAvailable,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.ReasonPodAvailableNotExists,
				Message: err.Error(),
			})
			return nil
		}
		return err
	}
	if isStatusConditionTrue(pod, corev1.PodReady) {
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodAvailable,
			Status: metav1.ConditionTrue,
			Reason: nyamberv1beta1.ReasonOK,
		})
	}
	if isStatusConditionTrue(pod, corev1.PodScheduled) {
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodAvailable,
			Status: metav1.ConditionTrue,
			Reason: nyamberv1beta1.ReasonOK,
		})
	}
	return nil
}

func (r *VirtualDCReconciler) finalize(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	if controllerutil.ContainsFinalizer(vdc, constants.FinalizerName) {
		if err := r.deletePod(ctx, vdc); err != nil {
			return err
		}
		controllerutil.RemoveFinalizer(vdc, constants.FinalizerName)
		err := r.Update(ctx, vdc)
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *VirtualDCReconciler) deletePod(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.PodNameSpace, Name: vdc.Name}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	ownerNs := pod.Labels[constants.OwnerNamespace]
	if ownerNs != vdc.Namespace {
		return nil
	}
	uid := pod.GetUID()
	cond := metav1.Preconditions{
		UID: &uid,
	}
	return r.Delete(ctx, pod, &client.DeleteOptions{
		Preconditions: &cond,
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	vdcPodHandler := func(o client.Object) []reconcile.Request {
		owner := o.GetLabels()[constants.OwnerNamespace]
		if owner == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: owner, Name: o.GetName()}}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nyamberv1beta1.VirtualDC{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(vdcPodHandler)).
		Complete(r)
}

func isStatusConditionTrue(pod *corev1.Pod, condition corev1.PodConditionType) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type != condition {
			continue
		}
		if cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
