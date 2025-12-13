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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	sweeperv1 "github.com/cloudflavor/sweeper-sentinel/api/v1"
)

var _ = Describe("SweeperSentinel Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "sweeper-sentinel"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "sweeper-sentinel",
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
						Namespace: "sweeper-sentinel",
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
})
