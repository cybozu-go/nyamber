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
	"sigs.k8s.io/controller-runtime/pkg/log"
)


func SetupAutoVirtualDCWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&nyamberv1beta1.AutoVirtualDC{}).
		WithValidator(&autoVirtualdcValidator{}).
		Complete()
}

type autoVirtualdcValidator struct{}


// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
//+kubebuilder:webhook:path=/validate-nyamber-cybozu-io-v1beta1-autovirtualdc,mutating=false,failurePolicy=fail,sideEffects=None,groups=nyamber.cybozu.io,resources=autovirtualdcs,verbs=create;update,versions=v1beta1,name=vautovirtualdc.kb.io,admissionReviewVersions=v1


// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (v autoVirtualdcValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	logger := log.FromContext(ctx)
	logger.Info("validate create", "name", obj.(*nyamberv1beta1.AutoVirtualDC).Name)


	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v autoVirtualdcValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	logger := log.FromContext(ctx)
	avdcName := oldObj.(*nyamberv1beta1.AutoVirtualDC).Name
	logger.Info("validate update", "name", avdcName)


	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v autoVirtualdcValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
