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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
)

// VirtualDCReconciler reconciles a VirtualDC object
type VirtualDCReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

const (
	namespace string = "default"
)

//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the VirtualDC object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.2/pkg/reconcile
func (r *VirtualDCReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	vdc := &nyamberv1beta1.VirtualDC{}
	if err := r.Get(ctx, req.NamespacedName, vdc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !meta.IsStatusConditionTrue(vdc.Status.Conditions, nyamberv1beta1.PodCreated) {
		if err := r.createPod(ctx, vdc); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.updateStatus(ctx, vdc); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *VirtualDCReconciler) createPod(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	// create a pod
	// check status of a pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vdc.Name,
			Namespace: namespace,
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
		if apierrors.IsAlreadyExists(err) {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.PodCreated,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.PodCreatedAlreadyExists,
				Message: err.Error(),
			})
		} else {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.PodCreated,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.PodCreatedFailed,
				Message: err.Error(),
			})
		}
		return err
	}
	meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
		Type:   nyamberv1beta1.PodCreated,
		Status: metav1.ConditionTrue,
	})
	return nil
}

func (r *VirtualDCReconciler) updateStatus(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: namespace, Name: vdc.Name}, pod); err != nil {
		if apierrors.IsNotFound(err) {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.PodAvailable,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.PodAvailableNotExists,
				Message: err.Error(),
			})
			return nil
		}
		return err
	}
	if isStatusConditionTrue(pod, corev1.PodReady) {
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.PodAvailable,
			Status: metav1.ConditionTrue,
		})
	}
	if isStatusConditionTrue(pod, corev1.PodScheduled) {
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.PodAvailable,
			Status: metav1.ConditionTrue,
		})
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nyamberv1beta1.VirtualDC{}).
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
