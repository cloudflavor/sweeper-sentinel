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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
			Namespace: "sweeper-sentinel", // TODO(user):Modify as needed
		}
		sweepersentinel := &sweeperv1.SweeperSentinel{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind SweeperSentinel")
			err := k8sClient.Get(ctx, typeNamespacedName, sweepersentinel)
			ttl := "10ms"
			if err != nil && errors.IsNotFound(err) {
				resource := &sweeperv1.SweeperSentinel{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "sweeper-sentinel",
					},
					Spec: sweeperv1.SweeperSentinelSpec{
						TTL: &ttl,
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
})
