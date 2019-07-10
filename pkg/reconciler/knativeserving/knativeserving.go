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

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	vk1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	mf "github.com/jcrossley3/manifestival"
	"github.com/knative/serving-operator/pkg/apis/serving/v1alpha1"
	listers "github.com/knative/serving-operator/pkg/client/listers/serving/v1alpha1"
	"github.com/knative/serving-operator/pkg/reconciler"
	"github.com/knative/serving-operator/version"
	appsv1listers "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

const (
	ReconcilerName = "KnativeServing"
)

type Reconciler struct {
	*reconciler.Base

	// Listers index properties about resources
	knativeServingLister          listers.KnativeServingLister
	deploymentLister              appsv1listers.DeploymentLister
	serviceLister                 corev1listers.ServiceLister
	config                        mf.Manifest
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

func (c *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		c.Logger.Errorf("invalid resource key: %s", key)
		return nil
	}
	logger := logging.FromContext(ctx)

	// Get the Route resource with this namespace/name.
	original, err := c.knativeServingLister.KnativeServings(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("route %q in work queue no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}
	// Don't modify the informers copy.
	knativeServing := original.DeepCopy()

	// Reconcile this copy of the route and then write back any status
	// updates regardless of whether the reconciliation errored out.
	reconcileErr := c.reconcile(ctx, knativeServing)
	if equality.Semantic.DeepEqual(original.Status, knativeServing.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err = c.updateStatus(knativeServing); err != nil {
		logger.Warnw("Failed to update route status", zap.Error(err))
		c.Recorder.Eventf(knativeServing, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for Route %q: %v", knativeServing.Name, err)
		return err
	}
	if reconcileErr != nil {
		c.Recorder.Event(knativeServing, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
		return reconcileErr
	}
	// TODO(mattmoor): Remove this after 0.7 cuts.
	// If the spec has changed, then assume we need an upgrade and issue a patch to trigger
	// the webhook to upgrade via defaulting.  Status updates do not trigger this due to the
	// use of the /status resource.
	if !equality.Semantic.DeepEqual(original.Spec, knativeServing.Spec) {
		routes := v1alpha1.SchemeGroupVersion.WithResource("routes")
		if err := c.MarkNeedsUpgrade(routes, knativeServing.Namespace, knativeServing.Name); err != nil {
			return err
		}
	}
	return nil
}

func (c *Reconciler) updateStatus(desired *v1alpha1.KnativeServing) (*v1alpha1.KnativeServing, error) {
	ks, err := c.knativeServingLister.KnativeServings(desired.Namespace).Get(desired.Name)
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
	return c.KnativeServingClientSet.ServingV1alpha1().KnativeServings(desired.Namespace).UpdateStatus(existing)
}

func (c *Reconciler) reconcile(ctx context.Context, ks *v1alpha1.KnativeServing) error {
	//logger := logging.FromContext(ctx)
	stages := []func(*v1alpha1.KnativeServing) error{
		c.initStatus,
		c.install,
		c.checkDeployments,
		c.deleteObsoleteResources,
	}

	for _, stage := range stages {
		if err := stage(ks); err != nil {
			return err
		}
	}
	return nil
}

// Initialize status conditions
func (c *Reconciler) initStatus(instance *v1alpha1.KnativeServing) error {
	log.V(1).Info("initStatus", "status", instance.Status)

	if len(instance.Status.Conditions) == 0 {
		instance.Status.InitializeConditions()
		if _, err := c.updateStatusInit(instance); err != nil {
			return err
		}
	}
	return nil
}

// Update the status subresource
func (c *Reconciler) updateStatusInit(instance *v1alpha1.KnativeServing) (*v1alpha1.KnativeServing, error) {
	gvk := instance.GroupVersionKind()
	defer instance.SetGroupVersionKind(gvk)
	existing := instance.DeepCopy()
	existing.Status = instance.Status

	if _, err := c.KnativeServingClientSet.ServingV1alpha1().KnativeServings(instance.Namespace).UpdateStatus(existing); err != nil {
		return nil, err
	}
	return nil, nil
}

// Apply the embedded resources
func (c *Reconciler) install(instance *v1alpha1.KnativeServing) error {
	log.V(1).Info("install", "status", instance.Status)
	if instance.Status.IsDeploying() {
		return nil
	}
	defer c.updateStatus(instance)

	//extensions, err := platforms.Extend(c.client, c.scheme)
	//if err != nil {
	//	return err
	//}
	//
	//err = c.config.Transform(extensions.Transform(c.scheme, instance)...)
	//if err == nil {
	//	err = extensions.PreInstall(instance)
	//	if err == nil {
	//		err = c.config.ApplyAll()
	//		if err == nil {
	//			err = extensions.PostInstall(instance)
	//		}
	//	}
	//}

	err := c.config.ApplyAll()
	if err != nil {
		instance.Status.MarkInstallFailed(err.Error())
		return err
	}

	// Update status
	instance.Status.Version = version.Version
	log.Info("Install succeeded", "version", version.Version)
	instance.Status.MarkInstallSucceeded()
	return nil
}

// Check for all deployments available
func (c *Reconciler) checkDeployments(instance *v1alpha1.KnativeServing) error {
	log.V(1).Info("checkDeployments", "status", instance.Status)
	defer c.updateStatus(instance)
	available := func(d *appsv1.Deployment) bool {
		for _, cd := range d.Status.Conditions {
			if cd.Type == appsv1.DeploymentAvailable && cd.Status == vk1.ConditionTrue {
				return true
			}
		}
		return false
	}
	deployment := &appsv1.Deployment{}
	for _, u := range c.config.Resources {
		if u.GetKind() == "Deployment" {
			if _, err := c.deploymentLister.Deployments(u.GetNamespace()).Get(u.GetName()); err != nil {
			//key := client.ObjectKey{Namespace: u.GetNamespace(), Name: u.GetName()}
			//if err := c.client.Get(context.TODO(), key, deployment); err != nil {
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
	}
	log.Info("All deployments are available")
	instance.Status.MarkDeploymentsAvailable()
	return nil
}

// Delete obsolete resources from previous versions
func (c *Reconciler) deleteObsoleteResources(instance *v1alpha1.KnativeServing) error {
	// istio-system resources from 0.3
	resource := &unstructured.Unstructured{}
	resource.SetNamespace("istio-system")
	resource.SetName("knative-ingressgateway")
	resource.SetAPIVersion("v1")
	resource.SetKind("Service")
	if err := c.config.Delete(resource, nil); err != nil {
		return err
	}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	if err := c.config.Delete(resource, nil); err != nil {
		return err
	}
	resource.SetAPIVersion("autoscaling/v1")
	resource.SetKind("HorizontalPodAutoscaler")
	if err := c.config.Delete(resource, nil); err != nil {
		return err
	}
	// config-controller from 0.5
	resource.SetNamespace(instance.GetNamespace())
	resource.SetName("config-controller")
	resource.SetAPIVersion("v1")
	resource.SetKind("ConfigMap")
	if err := c.config.Delete(resource, nil); err != nil {
		return err
	}
	return nil
}
