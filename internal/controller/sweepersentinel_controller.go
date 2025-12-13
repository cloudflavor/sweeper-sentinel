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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	sweeperv1 "github.com/cloudflavor/sweeper-sentinel/api/v1"
)

// SweeperSentinelReconciler reconciles a SweeperSentinel object
type SweeperSentinelReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	TickInterval time.Duration

	watchedGVKs map[string][]targetConfig
	watchMutex  sync.RWMutex

	targetsInitialized bool
}

type targetConfig struct {
	gvk schema.GroupVersionKind
}

const (
	ttlAnnotationKey  = "sweepers.sentinel.cloudflavor.io/ttl"
	enabledLabelKey   = "sweepers.sentinel.cloudflavor.io/enabled"
	enabledLabelValue = "true"
)

// +kubebuilder:rbac:groups=sweepers.sentinel.cloudflavor.io,resources=sweepersentinels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sweepers.sentinel.cloudflavor.io,resources=sweepersentinels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sweepers.sentinel.cloudflavor.io,resources=sweepersentinels/finalizers,verbs=update
func (r *SweeperSentinelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("request", req.NamespacedName)
	shouldSyncTargets := req.NamespacedName != (types.NamespacedName{}) || !r.targetsInitialized
	if shouldSyncTargets {
		if err := r.buildWatchConfig(ctx); err != nil {
			logger.Error(err, "failed to build watch configuration")
			return ctrl.Result{}, err
		}
		r.targetsInitialized = true
	}

	if err := r.sweepNamespaces(ctx); err != nil {
		logger.Error(err, "failed to sweep namespaces")
		return ctrl.Result{}, err
	}

	requeueAfter := r.TickInterval
	if requeueAfter <= 0 {
		requeueAfter = 10 * time.Second
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

func (r *SweeperSentinelReconciler) sweepNamespaces(ctx context.Context) error {
	log := log.FromContext(ctx)
	r.watchMutex.RLock()
	defer r.watchMutex.RUnlock()

	for namespace, targets := range r.watchedGVKs {
		for _, target := range targets {
			if err := r.sweepTarget(ctx, namespace, target); err != nil {
				log.Error(err, "failed to sweep target", "namespace", namespace, "gvk", target.gvk.String())
				return err
			}
		}
	}
	return nil
}

func (r *SweeperSentinelReconciler) sweepTarget(ctx context.Context, namespace string, target targetConfig) error {
	log := log.FromContext(ctx)
	ul := unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(target.gvk)

	if err := r.Client.List(ctx, &ul, client.InNamespace(namespace)); err != nil {
		return fmt.Errorf("list %s in namespace %s: %w", target.gvk.String(), namespace, err)
	}

	var sweepErr error
	for i := range ul.Items {
		resource := &ul.Items[i]
		if err := r.maybeDeleteExpired(ctx, resource); err != nil {
			log.Error(err, "failed to handle resource", "name", resource.GetName(), "namespace", resource.GetNamespace())
			sweepErr = err
		}
	}

	return sweepErr
}

func (r *SweeperSentinelReconciler) maybeDeleteExpired(ctx context.Context, resource *unstructured.Unstructured) error {
	if resource.GetLabels()[enabledLabelKey] != enabledLabelValue {
		return nil
	}

	ttl := resource.GetAnnotations()[ttlAnnotationKey]
	if ttl == "" {
		return nil
	}

	ttlConverted, err := parseTTL(ttl)
	if err != nil {
		return fmt.Errorf("parse ttl for %s/%s: %w", resource.GetNamespace(), resource.GetName(), err)
	}

	expires := resource.GetCreationTimestamp().Add(ttlConverted)
	if time.Now().After(expires) {
		if err := r.Client.Delete(ctx, resource); err != nil {
			return fmt.Errorf("delete expired resource %s/%s: %w", resource.GetNamespace(), resource.GetName(), err)
		}
		log.FromContext(ctx).Info("deleted expired resource", "name", resource.GetName(), "namespace", resource.GetNamespace(), "gvk", resource.GroupVersionKind().String(), "ttl", ttl)
	}

	return nil
}

func (r *SweeperSentinelReconciler) buildWatchConfig(ctx context.Context) error {
	log := log.FromContext(ctx)
	sentinels := sweeperv1.SweeperSentinelList{}
	if err := r.Client.List(ctx, &sentinels); err != nil {
		return err
	}

	updated := make(map[string][]targetConfig)
	for _, sentinel := range sentinels.Items {
		for _, target := range sentinel.Spec.Targets {
			gv, err := schema.ParseGroupVersion(target.APIVersion)
			if err != nil {
				log.Error(err, "failed to parse group version", "apiVersion", target.APIVersion, "kind", target.Kind)
				continue
			}
			updated[sentinel.Namespace] = append(updated[sentinel.Namespace], targetConfig{gvk: gv.WithKind(target.Kind)})
		}
	}

	r.watchMutex.Lock()
	defer r.watchMutex.Unlock()
	r.watchedGVKs = updated
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SweeperSentinelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.watchedGVKs = make(map[string][]targetConfig)
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
