/*
Copyright 2019 The Knative Authors.
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
	"reflect"

	mf "github.com/jcrossley3/manifestival"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	"knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	listers "knative.dev/serving-operator/pkg/client/listers/serving/v1alpha1"
	"knative.dev/serving-operator/pkg/reconciler"
	"knative.dev/serving-operator/pkg/reconciler/knativeserving/common"
	"knative.dev/serving-operator/version"
)

const (
	ReconcilerName = "KnativeServing"
)

var platforms common.Platforms

type Reconciler struct {
	*reconciler.Base

	// Listers index properties about resources
	knativeServingLister          listers.KnativeServingLister
	deploymentLister              appsv1listers.DeploymentLister
	serviceLister                 corev1listers.ServiceLister
	config                        mf.Manifest
	// TODO We keep this client in for transition to accommodate the old code. It will be removed later.
	client                        client.Client
	reconcileKnativeServing       ReconcileKnativeServing
	kubeClientSet                 kubernetes.Interface
	dynamicClientSet              dynamic.Interface
	scheme                        *runtime.Scheme
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		r.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	// Get the KnativeServing resource with this namespace/name.
	original, err := r.knativeServingLister.KnativeServings(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		r.Logger.Errorf("KnativeServing %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}
	// Don't modify the informers copy.
	knativeServing := original.DeepCopy()

	// Reconcile this copy of the route and then write back any status
	// updates regardless of whether the reconciliation errored out.
	reconcileErr := r.reconcile(ctx, knativeServing)
	if equality.Semantic.DeepEqual(original.Status, knativeServing.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err = r.updateStatus(ctx, knativeServing); err != nil {
		r.Logger.Warnw("Failed to update knativeServing status", zap.Error(err))
		r.Recorder.Eventf(knativeServing, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for KnativeServing %q: %v", knativeServing.Name, err)
		return err
	}
	if reconcileErr != nil {
		r.Recorder.Event(knativeServing, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
		return reconcileErr
	}
	return nil
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.KnativeServing) (*v1alpha1.KnativeServing, error) {
	ks, err := r.knativeServingLister.KnativeServings(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}
	// If there's nothing to update, just return.
	if reflect.DeepEqual(ks.Status, desired.Status) {
		return ks, nil
	}
	// Don't modify the informers copy
	existing := ks.DeepCopy()
	existing.Status = desired.Status
	return r.KnativeServingClientSet.ServingV1alpha1().KnativeServings(desired.Namespace).UpdateStatus(existing)
}

func (r *Reconciler) reconcile(ctx context.Context, ks *v1alpha1.KnativeServing) error {
	stages := []func(*v1alpha1.KnativeServing) error{
		r.initStatus,
		r.install,
		r.checkDeployments,
		r.deleteObsoleteResources,
	}

	for _, stage := range stages {
		if err := stage(ks); err != nil {
			return err
		}
	}
	return nil
}

// Initialize status conditions
func (r *Reconciler) initStatus(instance *v1alpha1.KnativeServing) error {
	if len(instance.Status.Conditions) == 0 {
		instance.Status.InitializeConditions()
		r.Logger.Info("Called initStatus. Let us do updateStatus to find the error")
		if err := r.updateStatusSubresource(instance); err != nil {
			return err
		}
	} else {
		r.Logger.Info("not calling of updateStatus in initStatus.")
	}
	return nil
}

// Update the status subresource
func (r *Reconciler) updateStatusSubresource(instance *v1alpha1.KnativeServing) error {
	r.Logger.Info("Let us do updateStatusSubresource to find the error")
	gvk := instance.GroupVersionKind()
	r.Logger.Info("GVK is %s.", gvk.String())
	defer instance.SetGroupVersionKind(gvk)

	if _, err := r.KnativeServingClientSet.ServingV1alpha1().KnativeServings(instance.Namespace).
		UpdateStatus(instance); err != nil {
		return err
	}
	return nil
}

// Apply the embedded resources
func (r *Reconciler) install(instance *v1alpha1.KnativeServing) error {
	if instance.Status.IsInstalled() {
		r.Logger.Info("called instacne deploying returned %s", instance.Status)
		return nil
	}
	r.Logger.Info("Called install. Let us do updateStatus to find the error status %s.", instance.Status)
	defer r.updateStatusSubresource(instance)

	r.Logger.Info("Called install extension")
	extensions, err := platforms.Extend(r.client, r.kubeClientSet, r.dynamicClientSet, r.scheme)
	if err != nil {
		return err
	}

	r.Logger.Info("Called install transform")
	err = r.config.Transform(extensions.Transform(r.scheme, instance)...)
	if err == nil {
		err = extensions.PreInstall(instance)
		if err == nil {
			err = r.config.ApplyAll()
			if err == nil {
				err = extensions.PostInstall(instance)
			}
		}
	}
	r.Logger.Info("Called install check error")
	if err != nil {
		instance.Status.MarkInstallFailed(err.Error())
		return err
	}

	// Update status
	instance.Status.Version = version.Version
	r.Logger.Info("Install succeeded, the version is %s", version.Version)
	instance.Status.MarkInstallSucceeded()
	return nil
}

func (r *Reconciler) checkDeployments(instance *v1alpha1.KnativeServing) error {
	r.Logger.Info("Called checkDeployments. Let us do updateStatus to find the error")
	defer r.updateStatusSubresource(instance)
	available := func(d *appsv1.Deployment) bool {
		for _, cd := range d.Status.Conditions {
			if cd.Type == appsv1.DeploymentAvailable && cd.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}

	for _, u := range r.config.Resources {
		if u.GetKind() == "Deployment" {
			if dep, err := r.deploymentLister.Deployments(u.GetNamespace()).Get(u.GetName()); err != nil {
				if apierrs.IsNotFound(err) {
					resource := apis.KindToResource(u.GroupVersionKind())
					_, err := r.dynamicClientSet.Resource(resource).Namespace(u.GetNamespace()).Create(&u,
						metav1.CreateOptions{})
					if err != nil {
						return err
					}
					return nil
				} else {
					return err
				}
			} else {
				if !available(dep) {
					r.Logger.Infof("The deployments are not available")
					instance.Status.MarkDeploymentsNotReady()
					return nil
				}
			}

		}
	}
	r.Logger.Infof("All deployments are available")
	instance.Status.MarkDeploymentsAvailable()
	return nil
}

// Delete obsolete resources from previous versions
func (r *Reconciler) deleteObsoleteResources(instance *v1alpha1.KnativeServing) error {
	// istio-system resources from 0.3
	resource := &unstructured.Unstructured{}
	resource.SetNamespace("istio-system")
	resource.SetName("knative-ingressgateway")
	resource.SetAPIVersion("v1")
	resource.SetKind("Service")
	if err := r.dynamicClientSet.Resource(apis.KindToResource(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).
		Delete(resource.GetName(), &metav1.DeleteOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			return err
		}
	}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	if err := r.dynamicClientSet.Resource(apis.KindToResource(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).
		Delete(resource.GetName(), &metav1.DeleteOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			return err
		}
	}
	resource.SetAPIVersion("autoscaling/v1")
	resource.SetKind("HorizontalPodAutoscaler")
	if err := r.dynamicClientSet.Resource(apis.KindToResource(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).
		Delete(resource.GetName(), &metav1.DeleteOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			return err
		}
	}
	// config-controller from 0.5
	resource.SetNamespace(instance.GetNamespace())
	resource.SetName("config-controller")
	resource.SetAPIVersion("v1")
	resource.SetKind("ConfigMap")
	if err := r.dynamicClientSet.Resource(apis.KindToResource(resource.GroupVersionKind())).Namespace(resource.GetNamespace()).
		Delete(resource.GetName(), &metav1.DeleteOptions{}); err != nil {
		if !apierrs.IsNotFound(err) {
			return err
		}
	}
	return nil
}
