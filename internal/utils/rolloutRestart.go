package utils

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rickslick/autorollout-operator/internal/consts"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const AnnotationFlipperRestartedAt = "flipper.ricktech.io/restartedAt"
const RolloutRestartAnnotation = "kubectl.kubernetes.io/restartedAt"
const RolloutManagedBy = "flipper.ricktech.io/managedBy"
const rolloutIntervalGroupName = "flipper.ricktech.io/IntervalGroup"
const errorUnsupportedKind = "unsupported Kind %v"

// HandleRolloutRestart handles rollout restart of object by patching with annotation
func HandleRolloutRestart(ctx context.Context, client ctrlclient.Client, obj ctrlclient.Object, managedByValue string, restartTimeInRFC3339 string) error {
	// log := log.FromContext(ctx)

	switch t := obj.(type) {
	case *appsv1.Deployment:
		patch := ctrlclient.StrategicMergeFrom(t.DeepCopy())
		if t.Spec.Template.ObjectMeta.Annotations == nil {
			t.Spec.Template.ObjectMeta.Annotations = make(map[string]string)
		}

		t.Annotations[RolloutManagedBy] = managedByValue
		if restartTimeInRFC3339 == "" {
			restartTimeInRFC3339 = time.Now().Format(time.RFC3339)
		}

		t.Annotations[AnnotationFlipperRestartedAt] = restartTimeInRFC3339
		t.Spec.Template.ObjectMeta.Annotations[RolloutRestartAnnotation] = restartTimeInRFC3339

		//TODO exponential backoff maybe use thirdparty lib ?
		//TODO wait for pods to be ready before proceeding and followed by annotation completedAt:time?
		return client.Patch(ctx, t, patch)
	default:
		return fmt.Errorf(errorUnsupportedKind, t)
	}
}

// HandleRolloutRestartList handles rollout restart for list of objects
// Currently support is only appsv1.DeploymentList
func HandleRolloutRestartList(ctx context.Context, k8sclient ctrlclient.Client, Objectlist client.ObjectList,
	recorder record.EventRecorder, flipperNamespacedName string, restartTimeInRFC3339 string) (failedObjList client.ObjectList, errs error) {
	var (
		err error
	)
	switch t := Objectlist.(type) {
	case *appsv1.DeploymentList:
		var failedRolloutDeploymentList = &appsv1.DeploymentList{}
		if t != nil {
			for _, obj := range t.Items {
				objCopied := obj.DeepCopy()
				if objCopied.Annotations == nil {
					objCopied.Annotations = make(map[string]string)
				}

				if err = HandleRolloutRestart(ctx, k8sclient, objCopied, flipperNamespacedName, restartTimeInRFC3339); err != nil {

					if client.IgnoreNotFound(err) != nil {

						errs = errors.Join(errs, err)
						failedRolloutDeploymentList.Items = append(failedRolloutDeploymentList.Items, obj)
						recorder.Eventf(objCopied, corev1.EventTypeWarning, consts.ReasonRolloutRestartFailed,
							"Rollout restart failed for target %#v: err=%s", objCopied, err)
					} else {
						recorder.Eventf(objCopied, corev1.EventTypeWarning, consts.ReasonRolloutRestartFailed,
							"Listed Object not found (mightbeDeleted) %#v: err=%s", objCopied, err)
					}
				} else {
					recorder.Eventf(objCopied, corev1.EventTypeNormal, consts.ReasonRolloutRestartTriggered,
						"Rollout restart triggered for %v", objCopied)
				}
			}
		}
		failedObjList = failedRolloutDeploymentList
	default:
		errs = fmt.Errorf(errorUnsupportedKind, t)
	}
	return

}

// func (r *FlipperReconciler) patchAllDeploymentRollout(ctx context.Context, deploymentList *appsv1.DeploymentList) ([]crdv1alpha1.DeploymentRolloutInfo, error) {
// 	var deploymentListInfo []v1alpha1.DeploymentRolloutInfo
// 	// Process the filtered deployments
// 	log := log.FromContext(ctx)
// 	now := time.Now()
// 	for _, deployment := range deploymentList.Items {
// 		log.Info("Patching deployment: %s", deployment.Name)

// 		if err := r.patchDeploymentRollout(ctx, &deployment, now); err != nil {
// 			log.Error(err, "Error in patching")
// 			deploymentListInfo = append(deploymentListInfo,
// 				v1alpha1.DeploymentRolloutInfo{Name: deployment.Name, Namespace: deployment.Namespace, TriggeredAt: now.String()})
// 			continue
// 		}
// 		deploymentListInfo = append(deploymentListInfo,
// 			v1alpha1.DeploymentRolloutInfo{Name: deployment.Name, Namespace: deployment.Namespace, TriggeredAt: now.String()})

// 	}

// 	return deploymentListInfo, nil
// }

// func (r *FlipperReconciler) updateDeploymentStatus(ctx context.Context, deploymentStatusList []crdv1alpha1.DeploymentRolloutInfo, deploymentName, deploymentNamespace string, status bool) error {

// 	for _, deploymentStatus := range deploymentStatusList {

// 		if deploymentStatus.Name == deploymentName && deploymentStatus.Namespace == deploymentNamespace {
// 			deploymentStatus.IsSuccess = status
// 			deploymentStatus.CompletedAt = time.Now().String()
// 		}
// 	}
// 	return nil
// }

// func (r *FlipperReconciler) patchDeploymentRollout(ctx context.Context, deployment *appsv1.Deployment, rolloutTime time.Time) error {
// 	//TODO metric for patch

// 	return nil
// }
// func (r *FlipperReconciler) getDeployment(ctx context.Context, deploymentName, deploymentNamspace string) (*appsv1.Deployment, error) {
// 	//TODO metric for get
// 	log := log.FromContext(ctx)
// 	deployment := &appsv1.Deployment{}
// 	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: deploymentNamspace, Name: deploymentName}, deployment); err != nil {
// 		log.Error(err, "Failed to get deployment", "deployment", deploymentName, "deployment namespace", deploymentNamspace)
// 		return nil, err
// 	}
// 	return deployment, nil
// }

// // VerifyRolloutRestart verifies if rollout restart is successful. It compares observedGeneration value change to determine of rolloutRestart a
// func (r *FlipperReconciler) VerifyRolloutRestart(ctx context.Context, deploymentName, deploymentNamspace string) bool {
// 	var err error
// 	var observedDeployment *appsv1.Deployment
// 	if observedDeployment, err = r.getDeployment(ctx, deploymentName, deploymentNamspace); err == nil {
// 		if observedDeployment.Status.ObservedGeneration == observedDeployment.Generation &&
// 			observedDeployment.Status.ReadyReplicas == *observedDeployment.Spec.Replicas &&
// 			observedDeployment.Status.UnavailableReplicas == 0 {
// 			return true
// 		}
// 	}
// 	return false
// }
// func (r *FlipperReconciler) VerifyDeploymentRollout(ctx context.Context, deploymentName, deploymentNamspace string) error {
// 	//TODO metric for verification
// 	log := log.FromContext(ctx)
// 	deployment := appsv1.Deployment{}
// 	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: deploymentNamspace, Name: deploymentName}, &deployment); err != nil {
// 		log.Error(err, "Failed to get deployment", "deployment", deploymentName, "deployment namespace", deploymentNamspace)
// 		return err
// 	}
// 	return nil
// }
