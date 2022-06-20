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
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
)

// VirtualDCReconciler reconciles a VirtualDC object
type VirtualDCReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=nyamber.nyamber.cybozu.io,resources=virtualdcs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nyamber.nyamber.cybozu.io,resources=virtualdcs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nyamber.nyamber.cybozu.io,resources=virtualdcs/finalizers,verbs=update

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
	err := r.ReconcilePod(ctx, req)

	// TODO(user): your logic here

	return ctrl.Result{}, err
}

func (r *VirtualDCReconciler) ReconcilePod(ctx context.Context, req ctrl.Request) error {
	dep := &appsv1.Deployment{}
	dep.SetNamespace("default")
	dep.SetName("sample")

	op, err := ctrl.CreateOrUpdate(ctx, r.Client, dep, func() error {
		dep.Spec.Replicas = pointer.Int32Ptr(2)
		dep.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "nginx"},
		}
		dep.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{"app": "nginx"},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:latest",
					},
				},
			},
		}
		return nil
	})
	if err != nil {
		return err
	}
	if op != controllerutil.OperationResultNone {
		fmt.Printf("Deployment %s\n", op)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&nyamberv1beta1.VirtualDC{}).
		Complete(r)
}
