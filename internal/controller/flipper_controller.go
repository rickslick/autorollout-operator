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
	"fmt"
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
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const maxWaitTime = 5 * time.Minute

// FlipperReconciler reconciles a Flipper object
type FlipperReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=crd.ricktech.io,resources=flippers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=crd.ricktech.io,resources=flippers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=crd.ricktech.io,resources=flippers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Flipper object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
//for each flipper CR
//step 1 : verify if the CR is already in processing
//step 2: If not processing get the list of deployments that match the criteria and trigger rollout retart
//Step 2.1 : Add the deployment information in the status of the CR  and mark as in processing with timestamp
//Step 2.2: Add infromation in the reconciler to invoke the same reconciler after 10 seconds
//Step 3: If it already in processing ,iterate through the list of deployments in the status and verify if rollout restart is done
//Step 4: If all of the deployments have done mark status of CR as done and exit without requeue
//Step 5: If atleast of the deployments havent done , mark status as in progress, and add backoff algo to increase interval of checks

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
		labelSelector         = labels.SelectorFromSet(rolloutSelectorLabels)
		listOptions           = &client.ListOptions{
			Namespace:     rolloutNameSpace,
			LabelSelector: labelSelector,
		}
		rolloutTime  = time.Now()
		nowInRFC3339 = rolloutTime.Format(time.RFC3339)
	)
	if rolloutInterval == 0 {
		rolloutInterval = consts.DEFAULT_FLIPPER_INTERVAL * time.Minute
	}

	log.Info("Checking Phase of Flipper CR ", "Phase", flipper.Status.Phase)
	switch flipper.Status.Phase {

	case "", crdv1alpha1.FlipPending:

		if err := r.Client.List(ctx, deploymentList, listOptions); client.IgnoreNotFound(err) != nil {
			log.Error(err, "Error in listing the deployments")
			flipper.Status.Reason = "Error in listing the deployments"
			r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartFailed", "Unable to list Deployments")
			flipper.Status.Phase = crdv1alpha1.FlipFailed
			return ctrl.Result{}, fmt.Errorf("unable to list objects for rollout restart")
		}

		if len(deploymentList.Items) == 0 {
			flipper.Status.Phase = crdv1alpha1.FlipPending
			if err := r.Status().Update(ctx, flipper); err != nil {
				r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartFailed", "Unable to update flipperStatus")
				return ctrl.Result{}, err
			}
			r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartPending", "No objects found")
			return ctrl.Result{Requeue: true}, nil
		}
	case crdv1alpha1.FlipFailed:
		deploymentList.Items = flipper.Status.FailedRolloutDeployments
	case crdv1alpha1.FlipSucceeded:
		if flipper.Status.LastScheduledRolloutTime.Add(rolloutInterval).Compare(time.Now()) <= 0 {
			flipper.Status.Phase = crdv1alpha1.FlipPending
			r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartInit", "Triggering next scheduled")
			r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartInit", "Moving state to pending")
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

	if failedObjList, err := utils.HandleRolloutRestartList(ctx, r.Client, deploymentList, r.Recorder, flipper.Namespace+"/"+flipper.Name, nowInRFC3339); err != nil {
		r.Recorder.Eventf(flipper, v1.EventTypeWarning, "RolloutRestartFailed", "Error", err.Error())
		flipper.Status.Phase = crdv1alpha1.FlipFailed
		if failedDeploymentList, ok := failedObjList.(*appsv1.DeploymentList); ok && failedDeploymentList != nil {
			flipper.Status.FailedRolloutDeployments = failedDeploymentList.Items
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

// if deploymentList != nil {

// 	filterDeploymentList = deploymentList
// for _, deployment := range deploymentList.Items {
// 	if deployment.Annotations != nil {
// 		//already associated with another CR
// 		if rolloutManger, ok := deployment.Annotations[utils.RolloutManagedBy]; ok  && rolloutManger != req.NamespacedName.String(){
// 			continue
// 		}
// 	}
// 	filterDeploymentList.Items = append(filterDeploymentList.Items, deployment)
// }
//}

// // FilterDeploymentList finds deployments that are not patched for rolloutRestart by the CR
// func (r *FlipperReconciler) Handle(ctx context.Context, flipper *v1alpha1.Flipper) (isDesiredState bool, filterDeploymentList *appsv1.DeploymentList, err error) {
// 	log := log.FromContext(ctx)
// 	rolloutSelectorLabels := flipper.Spec.Match.Labels
// 	rolloutNameSpace := flipper.Spec.Match.Namespace
// 	deploymentList := &appsv1.DeploymentList{}
// 	labelSelector := labels.SelectorFromSet(rolloutSelectorLabels)
// 	listOptions := &client.ListOptions{
// 		Namespace:     rolloutNameSpace,
// 		LabelSelector: labelSelector,
// 	}
// 	if err = r.Client.List(ctx, deploymentList, listOptions); err != nil {
// 		log.Error(err, "Error in listing the deployments")
// 		return
// 	}

// 	filterDeploymentList = &appsv1.DeploymentList{}

// 	if deploymentList != nil {
// 		for _, deployment := range deploymentList.Items {
// 			if deployment.Annotations != nil {
// 				if _, ok := deployment.Annotations[utils.RolloutManagedBy]; ok {
// 					continue
// 				}
// 			}
// 			filterDeploymentList.Items = append(filterDeploymentList.Items, deployment)
// 		}
// 	}
// 	if len(filterDeploymentList.Items) == 0 {
// 		isDesiredState = true
// 	}
// 	return
// }
