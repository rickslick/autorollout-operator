// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/rickslick/autorollout-operator/internal/consts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testCaseErrorPatch         = "error-patch"
	testCaseErrorPatchNotfound = "error-patch-apiNotFound"
)

func int32Ptr(i int32) *int32 {
	return &i
}
func generateSts() *appsv1.StatefulSet {

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mydeployment",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "myapp",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mycontainer",
							Image: "myimage",
						},
					},
				},
			},
		},
	}

}

type MockClient struct {
	client.Client
	TestCaseName string
}

func (m *MockClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	switch m.TestCaseName {
	case testCaseErrorPatch:
		return errors.New("simulated patch error")
	case testCaseErrorPatchNotfound:
		gvk := obj.GetObjectKind().GroupVersionKind()
		return apierrors.NewNotFound(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, obj.GetName())

	}
	return nil

}

func generateDeployment() *appsv1.Deployment {

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mydeployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "myapp",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "mycontainer",
							Image: "myimage",
						},
					},
				},
			},
		},
	}

}
func TestHandleRolloutRestartList(t *testing.T) {
	scheme := runtime.NewScheme()
	clientgoscheme.AddToScheme(scheme)
	ctx := context.TODO()
	clientFakeWithDeployment := fake.NewClientBuilder().WithObjects(generateDeployment()).Build()
	clientFakeWithStatefulSet := fake.NewClientBuilder().WithObjects(generateSts()).Build()

	mockClientForError := &MockClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}
	type args struct {
		ctx                   context.Context
		client                ctrlclient.Client
		Objectlist            client.ObjectList
		recorder              record.EventRecorder
		flipperNamespacedName string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{name: "nil-error", args: args{flipperNamespacedName: "redis/redisFlipper", ctx: ctx, client: clientFakeWithDeployment, recorder: record.NewFakeRecorder(10), Objectlist: &appsv1.DeploymentList{Items: []appsv1.Deployment{*generateDeployment()}}}},
		{name: "wrong-type", args: args{ctx: ctx, client: clientFakeWithStatefulSet, recorder: record.NewFakeRecorder(10), Objectlist: &appsv1.StatefulSetList{Items: []appsv1.StatefulSet{*generateSts()}}}, wantErr: true},
		{name: testCaseErrorPatch, args: args{ctx: ctx, client: mockClientForError, recorder: record.NewFakeRecorder(10), Objectlist: &appsv1.DeploymentList{Items: []appsv1.Deployment{*generateDeployment()}}}, wantErr: true},
		{name: testCaseErrorPatchNotfound, args: args{ctx: ctx, client: mockClientForError, recorder: record.NewFakeRecorder(10), Objectlist: &appsv1.DeploymentList{Items: []appsv1.Deployment{*generateDeployment()}}}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if MockClientInst, ok := tt.args.client.(*MockClient); ok {
				MockClientInst.TestCaseName = tt.name
				tt.args.client = MockClientInst
			}
			if _, err := HandleRolloutRestartList(tt.args.ctx, tt.args.client, tt.args.Objectlist, tt.args.recorder, tt.args.flipperNamespacedName, ""); (err != nil) != tt.wantErr {
				t.Errorf("HandleRolloutRestartList() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				if tt.wantErr == false && tt.name != testCaseErrorPatchNotfound {

					//verify if annotations are added
					switch objType := tt.args.Objectlist.(type) {
					case *appsv1.DeploymentList:
						for _, obj := range objType.Items {
							gotObj := &appsv1.Deployment{}
							tt.args.client.Get(ctx, types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}, gotObj)

							if gotObj.Spec.Template.Annotations == nil {
								t.Fatalf("HandleRolloutRestartList() expected %s to have annotation %s", obj.Name, consts.RolloutRestartAnnotation)
							}
							if _, ok := gotObj.Spec.Template.Annotations[consts.RolloutRestartAnnotation]; !ok {
								t.Fatalf("HandleRolloutRestartList() expected %s to have annotation %s", obj.Name, consts.RolloutRestartAnnotation)
							}
							if gotObj.Annotations == nil {
								t.Fatalf("HandleRolloutRestartList() expected %s to have annotation %s", obj.Name, consts.RolloutManagedBy)
							}
							if gotVal, ok := gotObj.Annotations[consts.RolloutManagedBy]; !ok || gotVal != tt.args.flipperNamespacedName {
								t.Fatalf("HandleRolloutRestartList() expected %s to have annotation %s with value %s but got %s", obj.Name, consts.RolloutManagedBy, tt.args.flipperNamespacedName, gotVal)
							}

						}
					}
				}
			}
		})
	}
}
