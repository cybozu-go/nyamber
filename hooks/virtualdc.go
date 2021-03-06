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
	"errors"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
func (v virtualdcValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	logger := log.FromContext(ctx)
	logger.Info("validate create", "name", obj.(*nyamberv1beta1.VirtualDC).Name)

	vdcs := &nyamberv1beta1.VirtualDCList{}
	if err := v.client.List(ctx, vdcs); err != nil {
		return err
	}

	for _, vdc := range vdcs.Items {
		if vdc.Name == obj.(*nyamberv1beta1.VirtualDC).Name {
			return errors.New("the name of VirtualDC resource conflicts")
		}
	}
	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v virtualdcValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	logger := log.FromContext(ctx)
	vdcName := oldObj.(*nyamberv1beta1.VirtualDC).Name
	logger.Info("validate update", "name", vdcName)

	var errs field.ErrorList
	oldSpec := oldObj.(*nyamberv1beta1.VirtualDC).Spec
	newSpec := newObj.(*nyamberv1beta1.VirtualDC).Spec

	if oldSpec.NecoBranch != newSpec.NecoBranch {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "necoBranch"), "the field is immutable"))
	}

	if oldSpec.NecoAppsBranch != newSpec.NecoAppsBranch {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "necoAppsBranch"), "the field is immutable"))
	}

	if oldSpec.SkipNecoApps != newSpec.SkipNecoApps {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "skipNecoApps"), "the field is immutable"))
	}

	if !equality.Semantic.DeepEqual(oldSpec.Command, newSpec.Command) {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "command"), "the field is immutable"))
	}

	if !equality.Semantic.DeepEqual(oldSpec.Resources, newSpec.Resources) {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "resources"), "the field is immutable"))
	}

	if len(errs) > 0 {
		err := apierrors.NewInvalid(schema.GroupKind{Group: nyamberv1beta1.GroupVersion.Group, Kind: "VirtualDC"}, vdcName, errs)
		logger.Error(err, "validation error", "name", vdcName)
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v virtualdcValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
