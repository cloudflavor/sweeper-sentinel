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
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"

	sweeperv1 "github.com/cloudflavor/sweeper-sentinel/api/v1"
)

// SweeperSentinelReconciler reconciles a SweeperSentinel object
type SweeperSentinelReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	TickInterval time.Duration

	watchedGVKs map[string][]schema.GroupVersionKind
	watchMutex  sync.RWMutex
}

// +kubebuilder:rbac:groups=sweeper.sweeper-sentinel.cloudflavor.io,resources=sweepersentinels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sweeper.sweeper-sentinel.cloudflavor.io,resources=sweepersentinels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sweeper.sweeper-sentinel.cloudflavor.io,resources=sweepersentinels/finalizers,verbs=update
func (r *SweeperSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	if err := r.fetchSentinels(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.sweep(ctx); err != nil {
		return ctrl.Result{}, err
	}

	// TODO: uncomment below, this is for now used for testing
	// return ctrl.Result{RequeueAfter: r.TickInterval}, nil
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

func (r *SweeperSentinelReconciler) sweep(ctx context.Context) error {
	for namespace, targets := range r.watchedGVKs {
		for _, target := range targets {
			r.fetchNamespaceResources(ctx, namespace, target)
		}
	}
	return nil
}

func (r *SweeperSentinelReconciler) fetchNamespaceResources(
	ctx context.Context, ns string, gvk schema.GroupVersionKind,
) error {
	ul := unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(gvk)

	r.Client.List(ctx, &ul, client.InNamespace(ns))
	for _, resource := range ul.Items {
		annotations := resource.GetAnnotations()
		if ttl := annotations["sweeper-sentinel.cloudflavor.io/ttl"]; ttl != "" {
			ttl := annotations["sweeper-sentinel.cloudflavor.io/ttl"]
			createdAt := resource.GetCreationTimestamp()
			fmt.Printf("%#v %s\n", createdAt, ttl)
			ttlConverted, err := parseTTL(ttl)

			if err != nil {
				fmt.Printf("failed to parse TTL:  %s", err)
				continue
			}

			expires := createdAt.Add(ttlConverted)
			currentTime := time.Now()

			if currentTime.After(expires) {
				if err := r.Client.Delete(ctx, &resource); err != nil {
					fmt.Printf("failed to delete resource: %s\n", err)
					continue
				} else {
					fmt.Printf("expired resource %s was deleted, TTL exceeded %s\n", resource.GetName(), ttl)
				}
			}
		}
	}
	return nil
}

func (r *SweeperSentinelReconciler) fetchSentinels(ctx context.Context) error {
	ul := sweeperv1.SweeperSentinelList{}
	err := r.Client.List(ctx, &ul)
	if err != nil {
		return err
	}

	for _, sentinel := range ul.Items {
		gvks := []schema.GroupVersionKind{}

		if _, ok := r.watchedGVKs[sentinel.Namespace]; !ok {
			r.watchedGVKs[sentinel.Namespace] = []schema.GroupVersionKind{}
		}

		for _, target := range sentinel.Spec.Targets {
			gv, err := schema.ParseGroupVersion(target.APIVersion)
			if err != nil {
				fmt.Printf("failed to parse group version: %s\n", err)
				continue
			}

			gvks = append(gvks, gv.WithKind(target.Kind))
		}

		r.watchedGVKs[sentinel.Namespace] = gvks
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SweeperSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.watchedGVKs = make(map[string][]schema.GroupVersionKind)
	return ctrl.NewControllerManagedBy(mgr).
		For(&sweeperv1.SweeperSentinel{}).
		Named("sweepersentinel").Complete(r)
}

func parseTTL(ttl string) (time.Duration, error) {
	if strings.HasSuffix(ttl, "d") {
		dayStr := strings.TrimSuffix(ttl, "d")
		days, err := strconv.Atoi(dayStr)
		if err != nil {
			return 0, err
		}
		return time.Hour * 24 * time.Duration(days), nil
	}
	return time.ParseDuration(ttl)
}
