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
	"github.com/rickslick/autorollout-operator/api/v1alpha1"
	crdv1alpha1 "github.com/rickslick/autorollout-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
	Context("When reconciling flipper object having label matching atleast one deployment", func() {
		const resourceName = "test-flipper"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()

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
			//adding existing deployment matching the label
			Expect(k8sClient.Create(ctx, generateDeployment())).To(Succeed())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &crdv1alpha1.Flipper{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, generateDeployment())).To(Succeed())
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

	Context("When reconciling flipper object list of deployments has error", func() {
		const resourceName = "test-flipper-t0"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()

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
			//adding existing deployment matching the label
			Expect(k8sClient.Create(ctx, generateDeployment())).To(Succeed())
		})

		AfterEach(func() {
			// Cleanup logic
			resource := &crdv1alpha1.Flipper{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, generateDeployment())).To(Succeed())
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

	Context("When reconciling flipper object having label matching atleast one deployment but patch returns error followed by success", func() {
		const resourceName = "test-flipper"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()

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
			Expect(k8sMockPatchClient.Create(ctx, generateDeployment())).To(Succeed())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &crdv1alpha1.Flipper{}
			err := k8sMockPatchClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sMockPatchClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sMockPatchClient.Delete(ctx, generateDeployment())).To(Succeed())
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

	Context("When reconciling flipper object(state: empty) and there are no deployments matching the criteria", func() {
		const resourceName = "test-flipper"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()

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
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &crdv1alpha1.Flipper{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
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
			Expect(result).Should(BeEquivalentTo(ctrl.Result{Requeue: true}), "Got different Result")
			resource := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipPending))
		})
	})

	Context("When reconciling flipper object(state: Pending) and there are no deployments matching the criteria", func() {
		const resourceName = "test-flipper"
		recorder := record.NewFakeRecorder(100)
		ctx := context.Background()

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
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &crdv1alpha1.Flipper{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
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
			Expect(result).Should(BeEquivalentTo(ctrl.Result{Requeue: true}), "Got different Result")
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
			Expect(k8sClient.Create(ctx, generateDeployment())).To(Succeed())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.

			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, flipper)).To(Succeed())
			Expect(k8sClient.Delete(ctx, generateDeployment())).To(Succeed())
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
			Expect(k8sClient.Create(ctx, generateDeployment())).To(Succeed())
		})

		AfterEach(func() {

			err := k8sClient.Get(ctx, typeNamespacedName, flipper)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Flipper")
			Expect(k8sClient.Delete(ctx, flipper)).To(Succeed())
			Expect(k8sClient.Delete(ctx, generateDeployment())).To(Succeed())
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
			Expect(k8sClient.Delete(ctx, generateDeployment())).To(Succeed())
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
			Expect(result).Should(BeEquivalentTo(ctrl.Result{Requeue: true}), "Got different Result")
			resource := &crdv1alpha1.Flipper{}
			err = k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			Expect(resource.Status.FailedRolloutDeployments).Should(BeNil())
			Expect(resource.Status.Phase).Should(BeEquivalentTo(crdv1alpha1.FlipPending))

			//adding existing deployment matching the label
			Expect(k8sClient.Create(ctx, generateDeployment())).To(Succeed())

			time.Sleep(10 * time.Millisecond)

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

func generateDeployment() *appsv1.Deployment {

	return &appsv1.Deployment{

		ObjectMeta: metav1.ObjectMeta{
			Name:      "mydeployment",
			Namespace: "default",
			Labels:    map[string]string{"app": "myapp"},
		},
		Spec: appsv1.DeploymentSpec{

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

// Define Flipper
func getFlipperObj() *v1alpha1.Flipper {

	return &v1alpha1.Flipper{
		Spec:       v1alpha1.FlipperSpec{Interval: "10m", Match: crdv1alpha1.MatchFilter{Labels: map[string]string{"app": "myapp"}}},
		ObjectMeta: metav1.ObjectMeta{Name: "flipperCR", Namespace: "default"}}
}

func getFakeClient(initObjs ...client.Object) (client.WithWatch, *runtime.Scheme, error) {

	s := scheme.Scheme
	if err := crdv1alpha1.AddToScheme(s); err != nil {
		return nil, nil, err
	}
	return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjs...).Build(), scheme.Scheme, nil
}

// func TestFlipperReconciler_HandleRolloutReconciler(t *testing.T) {

// 	fakeClient, scheme, err := getFakeClient(getFlipperObj(), generateDeployment())

// 	assert.NoError(t, err)
// 	assert.NotNil(t, fakeClient)
// 	assert.NotNil(t, scheme)
// 	ctx := context.TODO()

// 	assert.NoError(t, err)
// 	assert.Nil(t, err)
// 	type fields struct {
// 		Client   client.Client
// 		Scheme   *runtime.Scheme
// 		Recorder record.EventRecorder
// 	}
// 	type args struct {
// 		ctx     context.Context
// 		flipper *v1alpha1.Flipper
// 	}
// 	tests := []struct {
// 		name    string
// 		fields  fields
// 		args    args
// 		want    ctrl.Result
// 		wantErr bool
// 	}{
// 		{name: "init", fields: fields{Client: fakeClient, Recorder: record.NewFakeRecorder(10), Scheme: scheme},
// 			args: args{ctx: ctx, flipper: getFlipperObj()}},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			r := &FlipperReconciler{
// 				Client:   tt.fields.Client,
// 				Scheme:   tt.fields.Scheme,
// 				Recorder: tt.fields.Recorder,
// 			}
// 			r.Client.Create(context.Background(), getFlipperObj())
// 			got, err := r.Reconcile(tt.args.ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "flipperCR"}})
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("FlipperReconciler.HandleRolloutReconciler() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("FlipperReconciler.HandleRolloutReconciler() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
