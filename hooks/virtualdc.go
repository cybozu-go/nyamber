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

package hooks

import (
	"context"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&nyamberv1beta1.VirtualDC{}).
		WithValidator(&virtualdcValidator{client: mgr.GetClient()}).
		Complete()
}

//+kubebuilder:webhook:path=/validate-nyamber-cybozu-io-v1beta1-virtualdc,mutating=false,failurePolicy=fail,sideEffects=None,groups=nyamber.cybozu.io,resources=virtualdcs,verbs=create;update,versions=v1beta1,name=vvirtualdc.kb.io,admissionReviewVersions=v1

type virtualdcValidator struct {
	client client.Client
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *virtualdcValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	logger := log.FromContext(ctx)
	logger.Info("validate create", "name", obj.(*nyamberv1beta1.VirtualDC).Name)

	// TODO(user): fill in your validation logic upon object creation.
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *virtualdcValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	logger := log.FromContext(ctx)
	logger.Info("validate update", "name", oldObj.(*nyamberv1beta1.VirtualDC).Name)

	// TODO(user): fill in your validation logic upon object update.
	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *virtualdcValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	logger := log.FromContext(ctx)
	logger.Info("validate delete", "name", obj.(*nyamberv1beta1.VirtualDC).Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil
}
