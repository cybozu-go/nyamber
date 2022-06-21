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

	virtualdc := &nyamberv1beta1.VirtualDC{}
	if err := r.Get(ctx, req.NamespacedName, virtualdc); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if virtualdc.Status.Status == "success" {
		// get pod and check status
		return ctrl.Result{}, nil
	}
	if err := r.ReconcilePod(ctx, virtualdc); err != nil {
		return ctrl.Result{}, err
	}

	// TODO(user): your logic here
	return ctrl.Result{}, nil
}

func (r *VirtualDCReconciler) ReconcilePod(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	// create a pod
	// check status of a pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vdc.Name,
			Namespace: "default",
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
		vdc.Status.Status = "fail"
		return err
	}
	vdc.Status.Status = "success"
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nyamberv1beta1.VirtualDC{}).
		Complete(r)
}
