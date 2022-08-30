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
	"time"

	nyamberv1beta1 "github.com/cybozu-go/nyamber/api/v1beta1"
	"github.com/robfig/cron"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
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
	avdcName := obj.(*nyamberv1beta1.AutoVirtualDC).Name
	logger.Info("validate create", "name", avdcName)

	errs := v.validateCommon(obj)
	if len(errs) > 0 {
		err := apierrors.NewInvalid(schema.GroupKind{Group: nyamberv1beta1.GroupVersion.Group, Kind: "AutoVirtualDC"}, avdcName, errs)
		logger.Error(err, "validation error", "name", avdcName)
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (v autoVirtualdcValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	logger := log.FromContext(ctx)
	avdcName := newObj.(*nyamberv1beta1.AutoVirtualDC).Name
	logger.Info("validate update", "name", avdcName)

	errs := v.validateCommon(newObj)
	if len(errs) > 0 {
		err := apierrors.NewInvalid(schema.GroupKind{Group: nyamberv1beta1.GroupVersion.Group, Kind: "AutoVirtualDC"}, avdcName, errs)
		logger.Error(err, "validation error", "name", avdcName)
		return err
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (v autoVirtualdcValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (v autoVirtualdcValidator) validateCommon(obj runtime.Object) field.ErrorList {
	var errs field.ErrorList
	avdc :=  obj.(*nyamberv1beta1.AutoVirtualDC)

	if (avdc.Spec.StartSchedule != "" && avdc.Spec.StopSchedule == "") || (avdc.Spec.StartSchedule == "" && avdc.Spec.StopSchedule != ""){
		errs = append(errs, field.Forbidden(field.NewPath("spec", "startSchedule"), "specifing only one side is not allowed"))
		errs = append(errs, field.Forbidden(field.NewPath("spec", "stopSchedule"), "specifing only one side is not allowed"))
	}

	if avdc.Spec.StartSchedule != "" && avdc.Spec.StopSchedule != ""{
		specParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		_, err := specParser.Parse(avdc.Spec.StartSchedule)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("spec", "startSchedule"), avdc.Spec.StartSchedule, "the field can not be parsed"))
		}
		_, err = specParser.Parse(avdc.Spec.StopSchedule)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath("spec", "stopSchedule"), avdc.Spec.StopSchedule, "the field can not be parsed"))
		}
	}

	_, err := time.ParseDuration(avdc.Spec.TimeoutDuration)
	if err != nil {
		errs = append(errs, field.Invalid(field.NewPath("spec", "timeoutDuration"), avdc.Spec.TimeoutDuration, "the field can not be parsed"))
	}

	return errs

}