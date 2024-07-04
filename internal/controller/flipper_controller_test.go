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

	. "github.com/onsi/ginkgo/v2"
	crdv1alpha1 "github.com/rickslick/autorollout-operator/api/v1alpha1"
	"github.com/rickslick/autorollout-operator/internal/consts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	interval              = "10m"
	intervalInDuration, _ = time.ParseDuration(interval)
)

// Customized client which return error on Patch MockClient implements all functions of Client but overrides Patch
type MockPatchErrorClient struct {
	client.Client
}
type MockListErrorClient struct {
	client.Client
}

func (m *MockListErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {

	gvk := list.GetObjectKind().GroupVersionKind()
	return apierrors.NewForbidden(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, "", nil)

}
func (m *MockPatchErrorClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return apierrors.NewForbidden(schema.GroupResource{Group: gvk.Group, Resource: gvk.Kind}, obj.GetName(), nil)
}

var _ = Describe("Flipper Controller Tests", func() {
	Context("When reconciling a flipper object having label matching atleast one deployment", func() {
		const resourceName = "test-flipper"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()
		var depFake *appsv1.Deployment
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}

		flipper := &crdv1alpha1.Flipper{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Flipper")
			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			if err != nil && errors.IsNotFound(err) {
				flipper := &crdv1alpha1.Flipper{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: crdv1alpha1.FlipperSpec{Match: crdv1alpha1.MatchFilter{Labels: map[string]string{"app": "myapp"}}},
				}
				Expect(k8sClient.Create(ctx, flipper)).To(Succeed())
			}

			depFake = generateDeployment()
			Expect(k8sClient.Create(ctx, depFake)).To(Succeed())

			depFake.Status = appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			}
			Expect(k8sClient.Status().Update(context.Background(), depFake)).To(Succeed())

		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &crdv1alpha1.Flipper{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, depFake)).To(Succeed())
		})
		It("should successfully reconcile and add proper annotation to deployment", func() {
			By("Reconciling the created resource")
			controllerReconciler := &FlipperReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).Should(BeEquivalentTo(ctrl.Result{RequeueAfter: intervalInDuration}), "Got different Result")
			resource := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipSucceeded))
		})
	})

	Context("When reconciling flipper object but the  listing in reconciler got error", func() {
		const resourceName = "test-flipper-t0"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()
		var depFake *appsv1.Deployment
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}

		flipper := &crdv1alpha1.Flipper{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Flipper")
			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			if err != nil && errors.IsNotFound(err) {
				flipper := &crdv1alpha1.Flipper{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: crdv1alpha1.FlipperSpec{Match: crdv1alpha1.MatchFilter{Labels: map[string]string{"app": "myapp"}}},
				}
				Expect(k8sClient.Create(ctx, flipper)).To(Succeed())
			}
			depFake = generateDeployment()
			Expect(k8sClient.Create(ctx, depFake)).To(Succeed())

			depFake.Status = appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			}
			Expect(k8sClient.Status().Update(context.Background(), depFake)).To(Succeed())
		})

		AfterEach(func() {
			// Cleanup logic
			resource := &crdv1alpha1.Flipper{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, depFake)).To(Succeed())
		})
		It("should successfully reconcile and add proper annotation to deployment", func() {
			By("Reconciling the created resource")
			controllerReconciler := &FlipperReconciler{
				Client:   k8sMockListClient,
				Scheme:   k8sMockListClient.Scheme(),
				Recorder: recorder,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())
			resource := &crdv1alpha1.Flipper{}
			err = k8sMockPatchClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipFailed))
		})
	})

	Context("When reconciling flipper object having label matching atleast one deployment but patch returns error , then second round followed by success", func() {
		const resourceName = "test-flipper"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()
		var depFake *appsv1.Deployment
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}

		flipper := &crdv1alpha1.Flipper{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Flipper")
			err := k8sMockPatchClient.Get(ctx, typeNamespacedName, flipper)
			if err != nil && errors.IsNotFound(err) {
				flipper := &crdv1alpha1.Flipper{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: crdv1alpha1.FlipperSpec{Match: crdv1alpha1.MatchFilter{Labels: map[string]string{"app": "myapp"}}},
				}
				Expect(k8sMockPatchClient.Create(ctx, flipper)).To(Succeed())
			}
			//adding existing deployment matching the label
			depFake = generateDeployment()
			Expect(k8sClient.Create(ctx, depFake)).To(Succeed())

			depFake.Status = appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			}
			Expect(k8sClient.Status().Update(context.Background(), depFake)).To(Succeed())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &crdv1alpha1.Flipper{}
			err := k8sMockPatchClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sMockPatchClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sMockPatchClient.Delete(ctx, depFake)).To(Succeed())
		})
		It("should successfully reconcile and add proper annotation to deployment", func() {
			By("Reconciling the created resource")
			controllerReconciler := &FlipperReconciler{
				Client:   k8sMockPatchClient,
				Scheme:   k8sMockPatchClient.Scheme(),
				Recorder: recorder,
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(HaveOccurred())

			resource := &crdv1alpha1.Flipper{}
			err = k8sMockPatchClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).ShouldNot(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipFailed))

			/////////////////////////////////////////
			//trying again simulation by controller//
			////////////////////////////////////////
			controllerReconciler = &FlipperReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})

			Expect(err).NotTo(HaveOccurred())
			resource = &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipSucceeded))
			Expect(resource.Status.LastScheduledRolloutTime.String()).ShouldNot(BeEmpty())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())

		})
	})

	Context("When reconciling flipper object(state: empty) and there are no deployments matching the criteria(deployment exist but not ready)", func() {
		const resourceName = "test-flipper"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()
		var depFake *appsv1.Deployment
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}

		flipper := &crdv1alpha1.Flipper{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Flipper")
			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			if err != nil && errors.IsNotFound(err) {
				flipper := &crdv1alpha1.Flipper{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: crdv1alpha1.FlipperSpec{Match: crdv1alpha1.MatchFilter{Labels: map[string]string{"app": "myapp"}}},
				}
				Expect(k8sClient.Create(ctx, flipper)).To(Succeed())
				depFake = generateDeployment()
				Expect(k8sClient.Create(ctx, depFake)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &crdv1alpha1.Flipper{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sMockPatchClient.Delete(ctx, depFake)).To(Succeed())
		})
		It("should successfully reconcile and dont do anything but set status to pending", func() {
			By("Reconciling the created resource")
			controllerReconciler := &FlipperReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).Should(BeEquivalentTo(ctrl.Result{RequeueAfter: consts.DEFAULT_PENDING_WAIT_INTERVAL}), "Got different Result")
			resource := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipPending))
		})
	})

	Context("When reconciling flipper object object(state: Success) (restart of controller) without interval Lapse", func() {
		const resourceName = "test-flipper-first"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()
		flipper := &crdv1alpha1.Flipper{}
		var depFake *appsv1.Deployment
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		BeforeEach(func() {
			By("creating the custom resource for the Kind Flipper")
			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			if err != nil && errors.IsNotFound(err) {
				flipper = &crdv1alpha1.Flipper{

					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: crdv1alpha1.FlipperSpec{
						Interval: interval,
						Match:    crdv1alpha1.MatchFilter{Labels: map[string]string{"app": "myapp"}}},
				}
				Expect(k8sClient.Create(ctx, flipper)).To(Succeed())

				err = k8sClient.Get(ctx, typeNamespacedName, flipper)
				Expect(err).NotTo(HaveOccurred())

			}
			//adding existing deployment matching the label
			depFake = generateDeployment()
			Expect(k8sClient.Create(ctx, depFake)).To(Succeed())

			depFake.Status = appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			}
			Expect(k8sClient.Status().Update(context.Background(), depFake)).To(Succeed())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.

			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, flipper)).To(Succeed())
			Expect(k8sClient.Delete(ctx, depFake)).To(Succeed())
		})
		It("on second call it should rollout restart the deployment unless it crosses the interval duration(restart of controller Case)", func() {
			By("Reconciling twice")
			//first reconcile
			controllerReconciler := &FlipperReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).Should(BeEquivalentTo(ctrl.Result{RequeueAfter: intervalInDuration}), "Got different Result")
			resource := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipSucceeded))
			time.Sleep(10 * time.Millisecond)
			resultSecond, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipSucceeded))

			resourceAgain := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resourceAgain)
			Expect(err).NotTo(HaveOccurred())
			Expect(resultSecond.RequeueAfter > 0).Should(BeTrue())

			Expect(resultSecond.RequeueAfter < result.RequeueAfter).Should(BeTrueBecause("As it is called after 1st one"))
			Expect(resourceAgain.Status.LastScheduledRolloutTime).Should(BeEquivalentTo(resource.Status.LastScheduledRolloutTime), "Restart should be executed once per interval")

		})
	})
	Context("When reconciling flipper object object(state: Success) (restart of controller) with interval Lapse", func() {
		const resourceName = "test-flipper-first"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()
		flipper := &crdv1alpha1.Flipper{}
		var depFake *appsv1.Deployment
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		BeforeEach(func() {
			By("creating the custom resource for the Kind Flipper")
			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			if err != nil && errors.IsNotFound(err) {
				flipper = &crdv1alpha1.Flipper{

					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: crdv1alpha1.FlipperSpec{
						Interval: "5ms",
						Match:    crdv1alpha1.MatchFilter{Labels: map[string]string{"app": "myapp"}}},
				}
				Expect(k8sClient.Create(ctx, flipper)).To(Succeed())

				err = k8sClient.Get(ctx, typeNamespacedName, flipper)
				Expect(err).NotTo(HaveOccurred())

			}
			//adding existing deployment matching the label
			depFake = generateDeployment()
			Expect(k8sClient.Create(ctx, depFake)).To(Succeed())

			depFake.Status = appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			}
			Expect(k8sClient.Status().Update(context.Background(), depFake)).To(Succeed())
		})

		AfterEach(func() {

			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, flipper)).To(Succeed())
			Expect(k8sClient.Delete(ctx, depFake)).To(Succeed())
		})
		It("on second call it should rollout restart the deployment unless it crosses the interval duration(restart of controller Case)", func() {
			By("Reconciling twice")
			//first reconcile
			controllerReconciler := &FlipperReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).Should(BeEquivalentTo(ctrl.Result{RequeueAfter: 5 * time.Millisecond}), "Got different Result")
			resource := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipSucceeded))
			time.Sleep(10 * time.Millisecond)
			resultSecond, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipSucceeded))

			resourceAgain := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resourceAgain)
			Expect(err).NotTo(HaveOccurred())
			Expect(resultSecond.Requeue).Should(BeTrue())
			Expect(resourceAgain.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipPending))
			Expect(resourceAgain.Status.LastScheduledRolloutTime).Should(BeEquivalentTo(resource.Status.LastScheduledRolloutTime), "Should not change as rollout was not executed")

		})
	})
	Context("When reconciling flipper stage pending to success ", func() {
		const resourceName = "test-flipper-first"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()
		flipper := &crdv1alpha1.Flipper{}
		var depFake *appsv1.Deployment
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		BeforeEach(func() {
			By("creating the custom resource for the Kind Flipper")
			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			if err != nil && errors.IsNotFound(err) {
				flipper = &crdv1alpha1.Flipper{

					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: crdv1alpha1.FlipperSpec{
						Interval: interval,
						Match:    crdv1alpha1.MatchFilter{Labels: map[string]string{"app": "myapp"}}},
				}
				Expect(k8sClient.Create(ctx, flipper)).To(Succeed())

				err = k8sClient.Get(ctx, typeNamespacedName, flipper)
				Expect(err).NotTo(HaveOccurred())

			}

		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.

			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, flipper)).To(Succeed())
			Expect(k8sClient.Delete(ctx, depFake)).To(Succeed())
		})
		It("on second call it should rollout restart the deployment unless it crosses the interval duration(restart of controller Case)", func() {
			By("Reconciling twice")
			//first reconcile
			controllerReconciler := &FlipperReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: recorder,
			}

			result, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).Should(BeEquivalentTo(ctrl.Result{RequeueAfter: consts.DEFAULT_PENDING_WAIT_INTERVAL}), "Got different Result")
			resource := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipPending))

			//adding existing deployment matching the label
			depFake = generateDeployment()
			Expect(k8sClient.Create(ctx, depFake)).To(Succeed())

			depFake.Status = appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			}
			Expect(k8sClient.Status().Update(context.Background(), depFake)).To(Succeed())

			time.Sleep(10 * time.Millisecond)
			///////////// second reconciler /////////////
			resultSecond, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(resultSecond.RequeueAfter > 0).Should(BeTrue())

			resourceAgain := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resourceAgain)
			Expect(err).NotTo(HaveOccurred())
			Expect(resourceAgain.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resourceAgain.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipSucceeded))
			Expect(resourceAgain.Status.LastScheduledRolloutTime.String()).ShouldNot(BeEmpty())

		})
	})
})

func getPtrInt32(i int32) *int32 {
	return &i
}
func generateDeployment() *appsv1.Deployment {

	return &appsv1.Deployment{

		ObjectMeta: metav1.ObjectMeta{
			Name:      "mydeployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "myapp"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: getPtrInt32(1),
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
