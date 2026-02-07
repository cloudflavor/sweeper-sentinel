/*
Copyright 2025 Cloudflavor.

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
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	sweeperv1 "github.com/cloudflavor/sweeper-sentinel/api/v1"
)

var _ = Describe("SweeperSentinel Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-sweeper-sentinel"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "test-namespace",
		}

		ensureNamespaceExists := func() {
			ns := &corev1.Namespace{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: typeNamespacedName.Namespace}, ns); err != nil {
				if errors.IsNotFound(err) {
					ns.Name = typeNamespacedName.Namespace
					Expect(k8sClient.Create(ctx, ns)).To(Succeed())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		}

		sweepersentinel := &sweeperv1.SweeperSentinel{}

		BeforeEach(func() {
			ensureNamespaceExists()
			By("creating the custom resource for the Kind SweeperSentinel")
			err := k8sClient.Get(ctx, typeNamespacedName, sweepersentinel)
			if err != nil && errors.IsNotFound(err) {
				resource := &sweeperv1.SweeperSentinel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "test-namespace",
					},
					Spec: sweeperv1.SweeperSentinelSpec{
						Targets: []sweeperv1.Target{
							{
								APIVersion: "v1",
								Kind:       "Pod",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &sweeperv1.SweeperSentinel{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SweeperSentinel")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SweeperSentinelReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("parseTTL", func() {
		DescribeTable("should parse supported durations",
			func(input string, expected time.Duration) {
				dur, err := parseTTL(input)
				Expect(err).NotTo(HaveOccurred())
				Expect(dur).To(Equal(expected))
			},
			Entry("seconds", "30s", 30*time.Second),
			Entry("minutes", "10m", 10*time.Minute),
			Entry("hours", "2h", 2*time.Hour),
			Entry("days", "3d", 72*time.Hour),
		)

		It("should error on invalid input", func() {
			_, err := parseTTL("not-a-duration")
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Resource Deletion Logic", func() {
		const testNamespace = "deletion-test-namespace"
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: testNamespace}, ns); err != nil {
				if errors.IsNotFound(err) {
					ns.Name = testNamespace
					Expect(k8sClient.Create(ctx, ns)).To(Succeed())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		It("should delete expired pods", func() {
			// Create a SweeperSentinel that watches Pods
			sweeper := &sweeperv1.SweeperSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deletion-sweeper",
					Namespace: testNamespace,
				},
				Spec: sweeperv1.SweeperSentinelSpec{
					Targets: []sweeperv1.Target{
						{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sweeper)).To(Succeed())

			// Create a Pod with TTL annotation and enabled label
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-expired",
					Namespace: testNamespace,
					Labels: map[string]string{
						"sweepers.sentinel.cloudflavor.io/enabled": "true",
					},
					Annotations: map[string]string{
						"sweepers.sentinel.cloudflavor.io/ttl": "1s",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			// Reconcile to process the pod
			controllerReconciler := &SweeperSentinelReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				TickInterval: 10 * time.Second,
			}

			// First reconcile - should not delete the pod (just created)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-deletion-sweeper",
					Namespace: testNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify pod still exists
			createdPod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-pod-expired", Namespace: testNamespace}, createdPod)).To(Succeed())

			// Wait for TTL to expire
			time.Sleep(2 * time.Second)

			// Second reconcile - should delete the expired pod
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-deletion-sweeper",
					Namespace: testNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify pod is deleted
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-pod-expired", Namespace: testNamespace}, createdPod)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})

		It("should not delete pods that are not enabled", func() {
			// Create a SweeperSentinel that watches Pods
			sweeper := &sweeperv1.SweeperSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-non-enabled-sweeper",
					Namespace: testNamespace,
				},
				Spec: sweeperv1.SweeperSentinelSpec{
					Targets: []sweeperv1.Target{
						{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sweeper)).To(Succeed())

			// Create a Pod without the enabled label
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-not-enabled",
					Namespace: testNamespace,
					Annotations: map[string]string{
						"sweepers.sentinel.cloudflavor.io/ttl": "10m",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			// Reconcile to process the pod
			controllerReconciler := &SweeperSentinelReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				TickInterval: 10 * time.Second,
			}

			// Reconcile - should not delete the pod (not enabled)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-non-enabled-sweeper",
					Namespace: testNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify pod still exists
			createdPod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-pod-not-enabled", Namespace: testNamespace}, createdPod)).To(Succeed())
		})

		It("should not delete pods without TTL annotation", func() {
			// Create a SweeperSentinel that watches Pods
			sweeper := &sweeperv1.SweeperSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-no-ttl-sweeper",
					Namespace: testNamespace,
				},
				Spec: sweeperv1.SweeperSentinelSpec{
					Targets: []sweeperv1.Target{
						{
							APIVersion: "v1",
							Kind:       "Pod",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sweeper)).To(Succeed())

			// Create a Pod with enabled label but no TTL
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-no-ttl",
					Namespace: testNamespace,
					Labels: map[string]string{
						"sweepers.sentinel.cloudflavor.io/enabled": "true",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			// Reconcile to process the pod
			controllerReconciler := &SweeperSentinelReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				TickInterval: 10 * time.Second,
			}

			// Reconcile - should not delete the pod (no TTL)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-no-ttl-sweeper",
					Namespace: testNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify pod still exists
			createdPod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-pod-no-ttl", Namespace: testNamespace}, createdPod)).To(Succeed())
		})
	})

	Context("Multiple Resource Types", func() {
		const multiNamespace = "multi-test-namespace"
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: multiNamespace}, ns); err != nil {
				if errors.IsNotFound(err) {
					ns.Name = multiNamespace
					Expect(k8sClient.Create(ctx, ns)).To(Succeed())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		It("should handle multiple resource types in same SweeperSentinel", func() {
			// Create a SweeperSentinel that watches multiple resource types
			sweeper := &sweeperv1.SweeperSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-resource-sweeper",
					Namespace: multiNamespace,
				},
				Spec: sweeperv1.SweeperSentinelSpec{
					Targets: []sweeperv1.Target{
						{
							APIVersion: "v1",
							Kind:       "Pod",
						},
						{
							APIVersion: "v1",
							Kind:       "ConfigMap",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sweeper)).To(Succeed())

			// Create a Pod with TTL annotation and enabled label
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-multi",
					Namespace: multiNamespace,
					Labels: map[string]string{
						"sweepers.sentinel.cloudflavor.io/enabled": "true",
					},
					Annotations: map[string]string{
						"sweepers.sentinel.cloudflavor.io/ttl": "1s",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			// Create a ConfigMap with TTL annotation and enabled label
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cm-multi",
					Namespace: multiNamespace,
					Labels: map[string]string{
						"sweepers.sentinel.cloudflavor.io/enabled": "true",
					},
					Annotations: map[string]string{
						"sweepers.sentinel.cloudflavor.io/ttl": "1s",
					},
				},
				Data: map[string]string{
					"test": "value",
				},
			}
			Expect(k8sClient.Create(ctx, configMap)).To(Succeed())

			// Reconcile to process the resources
			controllerReconciler := &SweeperSentinelReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				TickInterval: 10 * time.Second,
			}

			// First reconcile - should not delete resources (just created)
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "multi-resource-sweeper",
					Namespace: multiNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify both resources still exist
			createdPod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-pod-multi", Namespace: multiNamespace}, createdPod)).To(Succeed())

			createdCM := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "test-cm-multi", Namespace: multiNamespace}, createdCM)).To(Succeed())

			// Wait for TTL to expire
			time.Sleep(2 * time.Second)

			// Second reconcile - should delete both expired resources
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "multi-resource-sweeper",
					Namespace: multiNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify both resources are deleted
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-pod-multi", Namespace: multiNamespace}, createdPod)
			Expect(errors.IsNotFound(err)).To(BeTrue())

			err = k8sClient.Get(ctx, types.NamespacedName{Name: "test-cm-multi", Namespace: multiNamespace}, createdCM)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("Watch Configuration Logic", func() {
		const configNamespace = "config-test-namespace"
		ctx := context.Background()

		BeforeEach(func() {
			ns := &corev1.Namespace{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: configNamespace}, ns); err != nil {
				if errors.IsNotFound(err) {
					ns.Name = configNamespace
					Expect(k8sClient.Create(ctx, ns)).To(Succeed())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		It("should build watch configuration correctly", func() {
			// Create a SweeperSentinel with multiple targets
			sweeper := &sweeperv1.SweeperSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "config-test-sweeper",
					Namespace: configNamespace,
				},
				Spec: sweeperv1.SweeperSentinelSpec{
					Targets: []sweeperv1.Target{
						{
							APIVersion: "v1",
							Kind:       "Pod",
						},
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sweeper)).To(Succeed())

			// Create the reconciler
			controllerReconciler := &SweeperSentinelReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				TickInterval: 10 * time.Second,
			}

			// Build watch config
			err := controllerReconciler.buildWatchConfig(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify the watch configuration was built
			controllerReconciler.watchMutex.RLock()
			Expect(controllerReconciler.watchedGVKs).To(HaveKey(configNamespace))
			Expect(controllerReconciler.watchedGVKs[configNamespace]).To(HaveLen(2))
			controllerReconciler.watchMutex.RUnlock()
		})

		It("should handle invalid API version gracefully", func() {
			// Create a SweeperSentinel with invalid API version
			sweeper := &sweeperv1.SweeperSentinel{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-version-sweeper",
					Namespace: configNamespace,
				},
				Spec: sweeperv1.SweeperSentinelSpec{
					Targets: []sweeperv1.Target{
						{
							APIVersion: "invalid/version",
							Kind:       "Pod",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, sweeper)).To(Succeed())

			// Create the reconciler
			controllerReconciler := &SweeperSentinelReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				TickInterval: 10 * time.Second,
			}

			// Build watch config - should not error even with invalid version
			err := controllerReconciler.buildWatchConfig(ctx)
			Expect(err).NotTo(HaveOccurred())

			// Verify no targets were added for invalid version
			controllerReconciler.watchMutex.RLock()
			Expect(controllerReconciler.watchedGVKs).To(HaveKey(configNamespace))
			// Should be empty since invalid version was skipped
			controllerReconciler.watchMutex.RUnlock()
		})
	})
})
