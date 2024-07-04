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

package controller

import (
	"context"
	"time"

	"github.com/rickslick/autorollout-operator/api/v1alpha1"
	crdv1alpha1 "github.com/rickslick/autorollout-operator/api/v1alpha1"
	"github.com/rickslick/autorollout-operator/internal/consts"
	"github.com/rickslick/autorollout-operator/internal/utils"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// FlipperReconciler reconciles a Flipper object
type FlipperReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=crd.ricktech.io,resources=flippers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.ricktech.io,resources=flippers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.ricktech.io,resources=flippers/finalizers,verbs=update
//+kubebuilder:rbac:groups=*,resources=deployments,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=*,resources=pods,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups=*,resources=events,verbs=get;list;watch;create;update;patch
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile

// for each flipperScheduler CR
// step 1: verify last run
func (r *FlipperReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	var err error
	//var duration time.Duration
	flipper := &v1alpha1.Flipper{}
	log.Info("Reconciling flipper %s", "NamespacedName", req.NamespacedName)
	if err = r.Get(ctx, req.NamespacedName, flipper); err != nil {
		// flipper object not found, it might have been deleted as soon as its created
		log.Error(err, "Unable to get the flipper object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("flipper Spec", "flipperSpec", flipper)

	return r.HandleRolloutReconciler(ctx, flipper)

}
func (r *FlipperReconciler) HandleRolloutReconciler(ctx context.Context, flipper *v1alpha1.Flipper) (ctrl.Result, error) {
	//Identify all deployments with the labels
	//Trigger rollout restart

	log := log.FromContext(ctx)
	var (
		rolloutSelectorLabels = flipper.Spec.Match.Labels
		rolloutNameSpace      = flipper.Spec.Match.Namespace
		rolloutInterval, _    = time.ParseDuration(flipper.Spec.Interval)
		deploymentList        = &appsv1.DeploymentList{}
		readyDeploymentList   = &appsv1.DeploymentList{
			Items: []appsv1.Deployment{},
		}
		labelSelector = labels.SelectorFromSet(rolloutSelectorLabels)
		listOptions   = &client.ListOptions{
			Namespace:     rolloutNameSpace,
			LabelSelector: labelSelector,
		}
		rolloutTime  = time.Now()
		nowInRFC3339 = rolloutTime.Format(time.RFC3339)
	)
	if rolloutInterval == 0 {
		//should not have occurred as webhook should have set default to 10m
		rolloutInterval = consts.DEFAULT_FLIPPER_INTERVAL
	}

	log.Info("Checking Phase of Flipper CR ", "Phase", flipper.Status.Phase)
	switch flipper.Status.Phase {

	case "", crdv1alpha1.FlipPending:

		if err := r.Client.List(ctx, deploymentList, listOptions); client.IgnoreNotFound(err) != nil {
			log.Error(err, "Error in listing the deployments")
			flipper.Status.Reason = "Error in listing the deployments"
			r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartFailed", "Unable to list Deployments")
			flipper.Status.Phase = crdv1alpha1.FlipFailed
			if err := r.Status().Update(ctx, flipper); err != nil {
				r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartFailed", "Unable to update flipperStatus")
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, err
		}
		// DeploymentList to store only ready deployments
		readyDeploymentList = &appsv1.DeploymentList{
			Items: []appsv1.Deployment{},
		}
		// Filter and get only ready deployments
		for _, deployment := range deploymentList.Items {
			if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
				readyDeploymentList.Items = append(readyDeploymentList.Items, deployment)
			} else {
				r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartWarning", "Deployment %s/%s not ready ignoring for rollout", deployment.GetNamespace(), deployment.GetName())
			}
		}

		if len(readyDeploymentList.Items) == 0 {
			flipper.Status.Phase = crdv1alpha1.FlipPending
			if err := r.Status().Update(ctx, flipper); err != nil {
				r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartFailed", "Unable to update flipperStatus")
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartPending", "No objects found")
			return ctrl.Result{RequeueAfter: consts.DEFAULT_PENDING_WAIT_INTERVAL}, nil
		}
	case crdv1alpha1.FlipFailed:
		for _, failedDeploymentInfo := range flipper.Status.FailedRolloutDeployments {
			failedDeployment := appsv1.Deployment{}
			if err := r.Client.Get(ctx, types.NamespacedName{Namespace: failedDeploymentInfo.Namespace, Name: failedDeploymentInfo.Name}, &failedDeployment); err == nil {
				// Filter and get only ready deployments

				if failedDeployment.Status.ReadyReplicas == *failedDeployment.Spec.Replicas {
					readyDeploymentList.Items = append(readyDeploymentList.Items, failedDeployment)
				} else {
					r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartWarning", "Deployment %s/%s not ready ignoring for rollout", failedDeployment.GetNamespace(), failedDeployment.GetName())
				}
			}
		}
	case crdv1alpha1.FlipSucceeded:
		if flipper.Status.LastScheduledRolloutTime.Add(rolloutInterval).Compare(time.Now()) <= 0 {
			flipper.Status.Phase = crdv1alpha1.FlipPending
			r.Recorder.Eventf(flipper, v1.EventTypeNormal, "RolloutRestartInit", "Triggering next scheduled")
			r.Recorder.Eventf(flipper, v1.EventTypeNormal, "RolloutRestartInit", "Moving state to pending")
			if err := r.Status().Update(ctx, flipper); err != nil {
				r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartFailed", "Unable to update flipperStatus")
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		} else {
			//case when restart of controller after success
			return ctrl.Result{RequeueAfter: time.Until(flipper.Status.LastScheduledRolloutTime.Add(rolloutInterval))}, nil
		}
	}
	//reset before adding in list
	flipper.Status.FailedRolloutDeployments = []crdv1alpha1.DeploymentInfo{}
	if failedObjList, err := utils.HandleRolloutRestartList(ctx, r.Client, deploymentList, r.Recorder, flipper.Namespace+"/"+flipper.Name, nowInRFC3339); err != nil {
		r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartFailed", "Error", err.Error())
		flipper.Status.Phase = crdv1alpha1.FlipFailed
		if failedDeploymentList, ok := failedObjList.(*appsv1.DeploymentList); ok && failedDeploymentList != nil {
			for _, failedDeployment := range failedDeploymentList.Items {
				flipper.Status.FailedRolloutDeployments = append(flipper.Status.FailedRolloutDeployments,
					crdv1alpha1.DeploymentInfo{Name: failedDeployment.Name, Namespace: failedDeployment.Namespace})
			}
		}
		errUpdate := r.Status().Update(ctx, flipper)
		if errUpdate != nil {
			return ctrl.Result{}, errUpdate
		}
		return ctrl.Result{}, err
	} else {
		r.Recorder.Eventf(flipper, v1.EventTypeNormal, "RolloutRestartSucceeded", "flipper %s succeeded", flipper.Name)
		flipper.Status.Phase = crdv1alpha1.FlipSucceeded
		flipper.Status.LastScheduledRolloutTime = metav1.NewTime(rolloutTime)
		err := r.Status().Update(ctx, flipper)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: rolloutInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FlipperReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{MaxConcurrentReconciles: 5}).
		For(&crdv1alpha1.Flipper{}).
		Complete(r)
}
