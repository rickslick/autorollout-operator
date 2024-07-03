/*
Copyright 2024.

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

package v1alpha1

import (
	"errors"
	"time"

	"github.com/rickslick/autorollout-operator/internal/consts"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var flipperlog = logf.Log.WithName("flipper-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *Flipper) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/mutate-crd-ricktech-io-v1alpha1-flipper,mutating=true,failurePolicy=fail,sideEffects=None,groups=crd.ricktech.io,resources=flippers,verbs=create;update,versions=v1alpha1,name=mflipper.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &Flipper{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *Flipper) Default() {
	flipperlog.Info("default", "name", r.Name)

	if r.Spec.Interval == "" {
		r.Spec.Interval = consts.DEFAULT_FLIPPER_INTERVAL.String()
	}
}

//+kubebuilder:webhook:path=/validate-crd-ricktech-io-v1alpha1-flipper,mutating=false,failurePolicy=fail,sideEffects=None,groups=crd.ricktech.io,resources=flippers,verbs=create;update,versions=v1alpha1,name=vflipper.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &Flipper{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *Flipper) ValidateCreate() (admission.Warnings, error) {
	flipperlog.Info("validate create", "name", r.Name)
	if err := r.validateFlipper(); err != nil {
		return admission.Warnings{err.Error()}, err
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *Flipper) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	flipperlog.Info("validate update", "name", r.Name)
	if err := r.validateFlipper(); err != nil {
		return admission.Warnings{err.Error()}, err
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *Flipper) ValidateDelete() (admission.Warnings, error) {
	flipperlog.Info("validate delete", "name", r.Name)
	return nil, nil
}
func (r *Flipper) validateInterval() error {
	flipperlog.Info("ValidateInterval", "name", r.Name)
	var err error
	if _, err = time.ParseDuration(r.Spec.Interval); err != nil {
		flipperlog.Error(err, "Interval is not in golang duration format")
	}
	return err
}
func (r *Flipper) validateLabels() error {
	flipperlog.Info("validateLabels", "name", r.Name)
	var err error
	if len(r.Spec.Match.Labels) == 0 {
		err = errors.New("no labels found")
		flipperlog.Error(err, "Labels absent")
	}
	return err

}

func (r *Flipper) validateFlipper() error {
	var allErrs error
	if err := r.validateInterval(); err != nil {
		allErrs = errors.Join(allErrs, err)
	}
	if err := r.validateLabels(); err != nil {
		allErrs = errors.Join(allErrs, err)
	}
	return allErrs
}
