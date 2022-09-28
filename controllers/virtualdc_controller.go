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
	"errors"
	"strings"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/cybozu-go/nyamber/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"sigs.k8s.io/yaml"
)

// VirtualDCReconciler reconciles a VirtualDC object
type VirtualDCReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	PodNamespace      string
	JobProcessManager JobProcessManager
}

//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=nyamber.cybozu.io,resources=virtualdcs/finalizers,verbs=update

//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete

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

	if !vdc.ObjectMeta.DeletionTimestamp.IsZero() {
		finalizeResult, err := r.finalize(ctx, vdc)
		if err != nil {
			logger.Error(err, "Finalize error")
			return ctrl.Result{}, err
		}
		return finalizeResult, nil
	}

	if !controllerutil.ContainsFinalizer(vdc, constants.FinalizerName) {
		controllerutil.AddFinalizer(vdc, constants.FinalizerName)
		err = r.Update(ctx, vdc)
		if err != nil {
			return ctrl.Result{}, err
		}
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

	if err := r.createService(ctx, vdc); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.JobProcessManager.Start(vdc); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("reconcile succeeded")
	return ctrl.Result{}, nil
}

func (r *VirtualDCReconciler) getPodTemplate(ctx context.Context) (*corev1.Pod, error) {
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: constants.ControllerNamespace, Name: constants.PodTemplateName}, cm); err != nil {
		return nil, err
	}
	pod := &corev1.Pod{}
	err := yaml.Unmarshal([]byte(cm.Data["pod-template"]), pod)
	if err != nil {
		return nil, err
	}

	if len(pod.Spec.Containers) == 0 {
		return nil, errors.New("pod.Spec.Containers are empty")
	}

	return pod, nil
}

func (r *VirtualDCReconciler) createPod(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	logger := log.FromContext(ctx)

	pod, err := r.getPodTemplate(ctx)
	if err != nil {
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:    nyamberv1beta1.TypePodCreated,
			Status:  metav1.ConditionFalse,
			Reason:  nyamberv1beta1.ReasonPodCreatedTemplateError,
			Message: err.Error(),
		})
		return err
	}

	pod.SetName(vdc.Name)
	pod.SetNamespace(r.PodNamespace)
	pod.SetLabels(mergeMap(pod.GetLabels(), map[string]string{
		constants.AppNameLabelKey:        constants.AppName,
		constants.AppComponentLabelKey:   constants.AppComponentRunner,
		constants.AppInstanceLabelKey:    vdc.Name,
		constants.LabelKeyOwnerNamespace: vdc.Namespace,
		constants.LabelKeyOwner:          vdc.Name,
	}))

	container := &pod.Spec.Containers[0]
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "NECO_BRANCH",
		Value: vdc.Spec.NecoBranch,
	})

	container.Args = []string{"neco_bootstrap:/scripts/neco-bootstrap"}

	if !vdc.Spec.SkipNecoApps {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "NECO_APPS_BRANCH",
			Value: vdc.Spec.NecoAppsBranch,
		})

		container.Args = append(
			container.Args,
			"neco_apps_bootstrap:/scripts/neco-apps-bootstrap",
		)
	}

	if len(vdc.Spec.Command) != 0 {
		container.Args = append(
			container.Args,
			"user_defined_command:"+strings.Join(vdc.Spec.Command, " "),
		)
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
		if err := r.Get(ctx, client.ObjectKey{Namespace: r.PodNamespace, Name: vdc.Name}, pod); err != nil {
			return err
		}
		owner := pod.Labels[constants.LabelKeyOwnerNamespace]
		if owner != vdc.Namespace {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.TypePodCreated,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.ReasonPodCreatedConflict,
				Message: "Resource with same name already exists in another namespace",
			})
			return nil
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

func (r *VirtualDCReconciler) createService(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	logger := log.FromContext(ctx)
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vdc.Name,
			Namespace: r.PodNamespace,
			Labels: map[string]string{
				constants.LabelKeyOwnerNamespace: vdc.Namespace,
				constants.LabelKeyOwner:          vdc.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				constants.LabelKeyOwnerNamespace: vdc.Namespace,
				constants.LabelKeyOwner:          vdc.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "status",
					Protocol:   corev1.ProtocolTCP,
					Port:       80,
					TargetPort: intstr.FromInt(constants.ListenPort),
				},
			},
		},
	}
	err := r.Create(ctx, svc)
	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.TypeServiceCreated,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.ReasonServiceCreatedFailed,
				Message: err.Error(),
			})
			return err
		}
		if err := r.Get(ctx, client.ObjectKey{Namespace: r.PodNamespace, Name: vdc.Name}, svc); err != nil {
			return err
		}
		owner := svc.Labels[constants.LabelKeyOwnerNamespace]
		if owner != vdc.Namespace {
			meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
				Type:    nyamberv1beta1.TypeServiceCreated,
				Status:  metav1.ConditionFalse,
				Reason:  nyamberv1beta1.ReasonServiceCreatedConflict,
				Message: "Resource with same name already exists in another namespace",
			})
			return nil
		}
		logger.Info("Service already exists")
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypeServiceCreated,
			Status: metav1.ConditionTrue,
			Reason: nyamberv1beta1.ReasonOK,
		})
		return nil
	}
	logger.Info("Service created")
	meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
		Type:   nyamberv1beta1.TypeServiceCreated,
		Status: metav1.ConditionTrue,
		Reason: nyamberv1beta1.ReasonOK,
	})
	return nil
}

func (r *VirtualDCReconciler) updateStatus(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) error {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.PodNamespace, Name: vdc.Name}, pod); err != nil {
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
	owner := pod.Labels[constants.LabelKeyOwnerNamespace]
	if owner != vdc.Namespace {
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:    nyamberv1beta1.TypePodAvailable,
			Status:  metav1.ConditionFalse,
			Reason:  nyamberv1beta1.ReasonPodAvailableNotAvailable,
			Message: "Resource with same name already exists in another namespace",
		})
		return nil
	}
	if !isStatusConditionTrue(pod, corev1.PodScheduled) {
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodAvailable,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodAvailableNotScheduled,
		})
		return nil
	}
	if !isStatusConditionTrue(pod, corev1.PodReady) {
		meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
			Type:   nyamberv1beta1.TypePodAvailable,
			Status: metav1.ConditionFalse,
			Reason: nyamberv1beta1.ReasonPodAvailableNotAvailable,
		})
		return nil
	}
	meta.SetStatusCondition(&vdc.Status.Conditions, metav1.Condition{
		Type:   nyamberv1beta1.TypePodAvailable,
		Status: metav1.ConditionTrue,
		Reason: nyamberv1beta1.ReasonOK,
	})
	return nil
}

func (r *VirtualDCReconciler) finalize(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("finalize start")
	if !controllerutil.ContainsFinalizer(vdc, constants.FinalizerName) {
		return ctrl.Result{}, nil
	}

	if err := r.JobProcessManager.Stop(vdc); err != nil {
		return ctrl.Result{}, err
	}

	requeueService, err := r.deleteService(ctx, vdc)
	if err != nil {
		return ctrl.Result{}, err
	}

	requeuePod, err := r.deletePod(ctx, vdc)
	if err != nil {
		return ctrl.Result{}, err
	}
	if requeueService || requeuePod {
		logger.Info("requeue has occured", "service", requeueService, "pod", requeuePod)
		return ctrl.Result{Requeue: true}, nil
	}

	controllerutil.RemoveFinalizer(vdc, constants.FinalizerName)
	err = r.Update(ctx, vdc)
	if err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("Finalize succeeded")
	return ctrl.Result{}, nil
}

func (r *VirtualDCReconciler) deletePod(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) (bool, error) {
	pod := &corev1.Pod{}
	err := r.Client.Get(ctx, client.ObjectKey{Name: vdc.Name, Namespace: r.PodNamespace}, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}

	ownerNs, ok := pod.Labels[constants.LabelKeyOwnerNamespace]
	if !ok || ownerNs != vdc.Namespace {
		return false, nil
	}
	if !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		return true, nil
	}
	uid := pod.GetUID()
	cond := metav1.Preconditions{
		UID: &uid,
	}
	if err := r.Delete(ctx, pod, &client.DeleteOptions{
		Preconditions: &cond,
	}); err != nil {
		return true, err
	}
	return true, nil
}

func (r *VirtualDCReconciler) deleteService(ctx context.Context, vdc *nyamberv1beta1.VirtualDC) (bool, error) {
	svc := &corev1.Service{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.PodNamespace, Name: vdc.Name}, svc); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return true, err
	}
	ownerNs, ok := svc.Labels[constants.LabelKeyOwnerNamespace]
	if !ok || ownerNs != vdc.Namespace {
		return false, nil
	}
	if !svc.ObjectMeta.DeletionTimestamp.IsZero() {
		return true, nil
	}
	uid := svc.GetUID()
	cond := metav1.Preconditions{
		UID: &uid,
	}
	if err := r.Delete(ctx, svc, &client.DeleteOptions{
		Preconditions: &cond,
	}); err != nil {
		return true, err
	}
	return true, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualDCReconciler) SetupWithManager(mgr ctrl.Manager) error {
	vdcHandler := func(o client.Object) []reconcile.Request {
		owner := o.GetLabels()[constants.LabelKeyOwnerNamespace]
		if owner == "" {
			return nil
		}
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: owner, Name: o.GetName()}}}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&nyamberv1beta1.VirtualDC{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, handler.EnqueueRequestsFromMapFunc(vdcHandler)).
		Watches(&source.Kind{Type: &corev1.Service{}}, handler.EnqueueRequestsFromMapFunc(vdcHandler)).
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

func mergeMap(m1, m2 map[string]string) map[string]string {
	m := make(map[string]string)
	for k, v := range m1 {
		m[k] = v
	}
	for k, v := range m2 {
		m[k] = v
	}
	if len(m) == 0 {
		return nil
	}
	return m
}
