/*
Copyright 2019 The Knative Authors

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

package knativeserving

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	clientset "knative.dev/serving-operator/pkg/client/clientset/versioned"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/logging"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	knsreconciler "knative.dev/serving-operator/pkg/client/injection/reconciler/serving/v1alpha1/knativeserving"
	listers "knative.dev/serving-operator/pkg/client/listers/serving/v1alpha1"
	"knative.dev/serving-operator/pkg/reconciler"
	"knative.dev/serving-operator/pkg/reconciler/knativeserving/common"
	"knative.dev/serving-operator/version"

	pkgreconciler "knative.dev/pkg/reconciler"
)

const (
	finalizerName  = "delete-knative-serving-manifest"
	creationChange = "creation"
	editChange     = "edit"
	deletionChange = "deletion"
)

var (
	role        mf.Predicate = mf.Any(mf.ByKind("ClusterRole"), mf.ByKind("Role"))
	rolebinding mf.Predicate = mf.Any(mf.ByKind("ClusterRoleBinding"), mf.ByKind("RoleBinding"))
)

// Reconciler implements controller.Reconciler for Knativeserving resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// knativeServingClientSet allows us to configure Serving objects
	knativeServingClientSet clientset.Interface
	// statsReporter reports reconciler's metrics.
	statsReporter reconciler.StatsReporter

	// Listers index properties about resources
	knativeServingLister listers.KnativeServingLister
	config               mf.Manifest
	servings             map[string]int64
	// Platform-specific behavior to affect the transform
	platform common.Platforms
}

// Check that our Reconciler implements controller.Reconciler
var _ knsreconciler.Interface = (*Reconciler)(nil)
var _ knsreconciler.Finalizer = (*Reconciler)(nil)

// FinalizeKind removes all resources after deletion of a KnativeServing.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *servingv1alpha1.KnativeServing) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	key, err := cache.MetaNamespaceKeyFunc(original)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	if _, ok := r.servings[key]; ok {
		delete(r.servings, key)
	}
	return r.delete(ctx, original)
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, original *servingv1alpha1.KnativeServing) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// Convert the namespace/name string into a distinct namespace and name
	key, err := cache.MetaNamespaceKeyFunc(original)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	// Keep track of the number and generation of KnativeServings in the cluster.
	newGen := original.Generation
	if oldGen, ok := r.servings[key]; ok {
		if newGen > oldGen {
			r.statsReporter.ReportKnativeservingChange(key, editChange)
		} else if newGen < oldGen {
			return fmt.Errorf("reconciling obsolete generation of KnativeServing %s: newGen = %d and oldGen = %d", key, newGen, oldGen)
		}
	} else {
		// No metrics are emitted when newGen > 1: the first reconciling of
		// a new operator on an existing KnativeServing resource.
		if newGen == 1 {
			r.statsReporter.ReportKnativeservingChange(key, creationChange)
		}
	}
	r.servings[key] = original.Generation

	// Reconcile this copy of the KnativeServing resource and then write back any status
	// updates regardless of whether the reconciliation errored out.
	err = r.reconcile(ctx, original)
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) reconcile(ctx context.Context, ks *servingv1alpha1.KnativeServing) error {
	logger := logging.FromContext(ctx)

	reqLogger := logger.With(zap.String("Request.Namespace", ks.Namespace)).With("Request.Name", ks.Name)
	reqLogger.Infow("Reconciling KnativeServing", "status", ks.Status)

	stages := []func(context.Context, *mf.Manifest, *servingv1alpha1.KnativeServing) error{
		r.ensureFinalizer,
		r.initStatus,
		r.install,
		r.checkDeployments,
		r.deleteObsoleteResources,
	}

	manifest, err := r.transform(ctx, ks)
	if err != nil {
		ks.Status.MarkInstallFailed(err.Error())
		return err
	}

	for _, stage := range stages {
		if err := stage(ctx, &manifest, ks); err != nil {
			return err
		}
	}
	reqLogger.Infow("Reconcile stages complete", "status", ks.Status)
	return nil
}

// Transform the resources
func (r *Reconciler) transform(ctx context.Context, instance *servingv1alpha1.KnativeServing) (mf.Manifest, error) {
	logger := logging.FromContext(ctx)

	logger.Debug("Transforming manifest")
	transforms, err := r.platform.Transformers(r.kubeClientSet, instance, logger)
	if err != nil {
		return mf.Manifest{}, err
	}
	return r.config.Transform(transforms...)
}

// Update the status subresource
func (r *Reconciler) updateStatus(instance *servingv1alpha1.KnativeServing) error {
	afterUpdate, err := r.knativeServingClientSet.OperatorV1alpha1().KnativeServings(instance.Namespace).UpdateStatus(instance)

	if err != nil {
		return err
	}
	// TODO: We shouldn't rely on mutability and return the updated entities from functions instead.
	afterUpdate.DeepCopyInto(instance)
	return nil
}

// Initialize status conditions
func (r *Reconciler) initStatus(ctx context.Context, _ *mf.Manifest, instance *servingv1alpha1.KnativeServing) error {
	logger := logging.FromContext(ctx)

	logger.Debug("Initializing status")
	if len(instance.Status.Conditions) == 0 {
		instance.Status.InitializeConditions()
		if err := r.updateStatus(instance); err != nil {
			return err
		}
	}
	return nil
}

// Apply the manifest resources
func (r *Reconciler) install(ctx context.Context, manifest *mf.Manifest, instance *servingv1alpha1.KnativeServing) error {
	logger := logging.FromContext(ctx)

	logger.Debug("Installing manifest")
	// The Operator needs a higher level of permissions if it 'bind's non-existent roles.
	// To avoid this, we strictly order the manifest application as (Cluster)Roles, then
	// (Cluster)RoleBindings, then the rest of the manifest.
	if err := manifest.Filter(role).Apply(); err != nil {
		instance.Status.MarkInstallFailed(err.Error())
		return err
	}
	if err := manifest.Filter(rolebinding).Apply(); err != nil {
		instance.Status.MarkInstallFailed(err.Error())
		return err
	}
	if err := manifest.Apply(); err != nil {
		instance.Status.MarkInstallFailed(err.Error())
		return err
	}
	instance.Status.MarkInstallSucceeded()
	instance.Status.Version = version.Version
	return nil
}

// Check for all deployments available
func (r *Reconciler) checkDeployments(ctx context.Context, manifest *mf.Manifest, instance *servingv1alpha1.KnativeServing) error {
	logger := logging.FromContext(ctx)

	logger.Debug("Checking deployments")
	available := func(d *appsv1.Deployment) bool {
		for _, c := range d.Status.Conditions {
			if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}
	for _, u := range manifest.Filter(mf.ByKind("Deployment")).Resources() {
		deployment, err := r.kubeClientSet.AppsV1().Deployments(u.GetNamespace()).Get(u.GetName(), metav1.GetOptions{})
		if err != nil {
			instance.Status.MarkDeploymentsNotReady()
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if !available(deployment) {
			instance.Status.MarkDeploymentsNotReady()
			return nil
		}
	}
	instance.Status.MarkDeploymentsAvailable()
	return nil
}

// ensureFinalizer attaches a "delete manifest" finalizer to the instance
func (r *Reconciler) ensureFinalizer(ctx context.Context, manifest *mf.Manifest, instance *servingv1alpha1.KnativeServing) error {
	for _, finalizer := range instance.GetFinalizers() {
		if finalizer == finalizerName {
			return nil
		}
	}
	instance.SetFinalizers(append(instance.GetFinalizers(), finalizerName))
	_, err := r.knativeServingClientSet.OperatorV1alpha1().KnativeServings(instance.Namespace).Update(instance)
	return err
}

// delete all the resources in the release manifest
func (r *Reconciler) delete(ctx context.Context, instance *servingv1alpha1.KnativeServing) error {
	logger := logging.FromContext(ctx)

	if len(instance.GetFinalizers()) == 0 || instance.GetFinalizers()[0] != finalizerName {
		return nil
	}
	logger.Info("Deleting resources")
	var RBAC = mf.Any(mf.ByKind("Role"), mf.ByKind("ClusterRole"), mf.ByKind("RoleBinding"), mf.ByKind("ClusterRoleBinding"))
	if len(r.servings) == 0 {
		if err := r.config.Filter(mf.ByKind("Deployment")).Delete(); err != nil {
			return err
		}
		if err := r.config.Filter(mf.NoCRDs, mf.None(RBAC)).Delete(); err != nil {
			return err
		}
		// Delete Roles last, as they may be useful for human operators to clean up.
		if err := r.config.Filter(RBAC).Delete(); err != nil {
			return err
		}
	}
	// The deletionTimestamp might've changed. Fetch the resource again.
	refetched, err := r.knativeServingLister.KnativeServings(instance.Namespace).Get(instance.Name)
	if err != nil {
		return err
	}
	refetched.SetFinalizers(refetched.GetFinalizers()[1:])
	_, err = r.knativeServingClientSet.OperatorV1alpha1().KnativeServings(refetched.Namespace).Update(refetched)
	return err
}

// Delete obsolete resources from previous versions
func (r *Reconciler) deleteObsoleteResources(ctx context.Context, manifest *mf.Manifest, instance *servingv1alpha1.KnativeServing) error {
	// istio-system resources from 0.3
	resource := &unstructured.Unstructured{}
	resource.SetNamespace("istio-system")
	resource.SetName("knative-ingressgateway")
	resource.SetAPIVersion("v1")
	resource.SetKind("Service")
	if err := manifest.Client.Delete(resource); err != nil {
		return err
	}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	if err := manifest.Client.Delete(resource); err != nil {
		return err
	}
	resource.SetAPIVersion("autoscaling/v1")
	resource.SetKind("HorizontalPodAutoscaler")
	if err := manifest.Client.Delete(resource); err != nil {
		return err
	}
	// config-controller from 0.5
	resource.SetNamespace(instance.GetNamespace())
	resource.SetName("config-controller")
	resource.SetAPIVersion("v1")
	resource.SetKind("ConfigMap")
	if err := manifest.Client.Delete(resource); err != nil {
		return err
	}
	return nil
}
