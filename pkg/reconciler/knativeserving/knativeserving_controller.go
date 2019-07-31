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
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/injection/clients/kubeclient"
	servingclient "knative.dev/serving-operator/pkg/client/injection/client"

	mf "github.com/jcrossley3/manifestival"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	serving "knative.dev/serving-operator/pkg/client/clientset/versioned"
	"knative.dev/serving-operator/pkg/reconciler/knativeserving/common"
	"knative.dev/serving-operator/version"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	operand = "knative-serving"
)

var (
	recursive = flag.Bool("recursive", false,
		"If filename is a directory, process all manifests recursively")
	log = logf.Log.WithName("controller_knativeserving")
	// Platform-specific behavior to affect the installation
	platforms common.Platforms
)

// Add creates a new KnativeServing Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, clientConfig *rest.Config) error {
	return add(mgr, newReconciler(mgr, clientConfig))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, clientConfig *rest.Config) reconcile.Reconciler {
	return &ReconcileKnativeServing{client: mgr.GetClient(), scheme: mgr.GetScheme(), clientConfig: clientConfig}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("knativeserving-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource KnativeServing
	err = c.Watch(&source.Kind{Type: &servingv1alpha1.KnativeServing{}}, &handler.EnqueueRequestForObject{}, predicate.ResourceVersionChangedPredicate{})
	if err != nil {
		return err
	}

	// Watch child deployments for availability
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &servingv1alpha1.KnativeServing{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileKnativeServing{}

// ReconcileKnativeServing reconciles a KnativeServing object
type ReconcileKnativeServing struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver

	kubeClientSet    kubernetes.Interface
	dynamicClientSet dynamic.Interface
	servingClient    serving.Interface
	client           client.Client
	scheme           *runtime.Scheme
	config           mf.Manifest
	clientConfig     *rest.Config
}

// Create manifestival resources and KnativeServing, if necessary
func (r *ReconcileKnativeServing) InjectClient(c client.Client) error {
	koDataDir := os.Getenv("KO_DATA_PATH")
	m, err := mf.NewManifest(filepath.Join(koDataDir, "knative-serving/"), *recursive, r.clientConfig)
	if err != nil {
		log.Error(err, "Failed to load manifest")
		return err
	}
	r.config = m

	ctx, _ := injection.Default.SetupInformers(context.TODO(), r.clientConfig)

	r.kubeClientSet = kubeclient.Get(ctx)
	r.dynamicClientSet = dynamicclient.Get(ctx)
	r.servingClient = servingclient.Get(ctx)
	return r.ensureKnativeServing()
}

// Reconcile reads that state of the cluster for a KnativeServing object and makes changes based on the state read
// and what is in the KnativeServing.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileKnativeServing) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling KnativeServing")

	// Fetch the KnativeServing instance
	instance, err := r.servingClient.ServingV1alpha1().KnativeServings(request.Namespace).Get(request.Name, metav1.GetOptions{})
	// TODO(markusthoemmes): This is annoying...
	instance.SetGroupVersionKind(servingv1alpha1.SchemeGroupVersion.WithKind("KnativeServing"))
	if err != nil {
		if errors.IsNotFound(err) {
			if isInteresting(request) {
				r.config.DeleteAll(&metav1.DeleteOptions{})
			}
			reqLogger.V(1).Info("No KnativeServing")
			return reconcile.Result{}, nil
		}
		reqLogger.Error(err, "Error getting KnativeServing")
		return reconcile.Result{}, err
	}

	if !isInteresting(request) {
		reqLogger.V(1).Info("Not interesting KnativeServing", "request", request.String)
		return reconcile.Result{}, r.ignore(instance)
	}

	stages := []func(*servingv1alpha1.KnativeServing) error{
		r.initStatus,
		r.install,
		r.checkDeployments,
		r.deleteObsoleteResources,
	}

	for _, stage := range stages {
		if err := stage(instance); err != nil {
			return reconcile.Result{}, err
		}
	}
	return reconcile.Result{}, nil
}

// Initialize status conditions
func (r *ReconcileKnativeServing) initStatus(instance *servingv1alpha1.KnativeServing) error {
	log.V(1).Info("initStatus", "status", instance.Status)

	if len(instance.Status.Conditions) == 0 {
		instance.Status.InitializeConditions()
		if err := r.updateStatus(instance); err != nil {
			return err
		}
	}
	return nil
}

// Update the status subresource
func (r *ReconcileKnativeServing) updateStatus(instance *servingv1alpha1.KnativeServing) error {
	afterUpdate, err := r.servingClient.ServingV1alpha1().KnativeServings(instance.Namespace).UpdateStatus(instance)
	if err != nil {
		return err
	}
	// TODO(markusthoemmes): We shouldn't rely on mutability and return the updated entities from functions instead.
	afterUpdate.DeepCopyInto(instance)
	return nil
}

// Apply the embedded resources
func (r *ReconcileKnativeServing) install(instance *servingv1alpha1.KnativeServing) error {
	log.V(1).Info("install", "status", instance.Status)
	if instance.Status.IsDeploying() {
		return nil
	}
	defer r.updateStatus(instance)

	extensions, err := platforms.Extend(r.client, r.kubeClientSet, r.dynamicClientSet, r.scheme)
	if err != nil {
		return err
	}

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
func (r *ReconcileKnativeServing) checkDeployments(instance *servingv1alpha1.KnativeServing) error {
	log.V(1).Info("checkDeployments", "status", instance.Status)
	defer r.updateStatus(instance)
	available := func(d *appsv1.Deployment) bool {
		for _, c := range d.Status.Conditions {
			if c.Type == appsv1.DeploymentAvailable && c.Status == v1.ConditionTrue {
				return true
			}
		}
		return false
	}
	for _, u := range r.config.Resources {
		if u.GetKind() == "Deployment" {
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
	}
	log.Info("All deployments are available")
	instance.Status.MarkDeploymentsAvailable()
	return nil
}

// Delete obsolete resources from previous versions
func (r *ReconcileKnativeServing) deleteObsoleteResources(instance *servingv1alpha1.KnativeServing) error {
	// istio-system resources from 0.3
	resource := &unstructured.Unstructured{}
	resource.SetNamespace("istio-system")
	resource.SetName("knative-ingressgateway")
	resource.SetAPIVersion("v1")
	resource.SetKind("Service")
	if err := r.config.Delete(resource, &metav1.DeleteOptions{}); err != nil {
		return err
	}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	if err := r.config.Delete(resource, &metav1.DeleteOptions{}); err != nil {
		return err
	}
	resource.SetAPIVersion("autoscaling/v1")
	resource.SetKind("HorizontalPodAutoscaler")
	if err := r.config.Delete(resource, &metav1.DeleteOptions{}); err != nil {
		return err
	}
	// config-controller from 0.5
	resource.SetNamespace(instance.GetNamespace())
	resource.SetName("config-controller")
	resource.SetAPIVersion("v1")
	resource.SetKind("ConfigMap")
	if err := r.config.Delete(resource, &metav1.DeleteOptions{}); err != nil {
		return err
	}
	return nil
}

// Because it's effectively cluster-scoped, we only care about a
// single, named resource: knative-serving/knative-serving
func isInteresting(request reconcile.Request) bool {
	return request.Namespace == operand && request.Name == operand
}

// Reflect our ignorance in the KnativeServing status
func (r *ReconcileKnativeServing) ignore(instance *servingv1alpha1.KnativeServing) (err error) {
	err = r.initStatus(instance)
	if err == nil {
		msg := fmt.Sprintf("The only KnativeServing resource that matters is %s/%s", operand, operand)
		instance.Status.MarkIgnored(msg)
		err = r.updateStatus(instance)
	}
	return
}

// If we can't find knative-serving/knative-serving, create it
func (r *ReconcileKnativeServing) ensureKnativeServing() error {
	koDataDir := os.Getenv("KO_DATA_PATH")
	const path = "serving_v1alpha1_knativeserving_cr.yaml"

	if _, err := r.servingClient.ServingV1alpha1().KnativeServings(operand).Get(operand, metav1.GetOptions{}); err != nil {
		manifest, err := mf.NewManifest(filepath.Join(koDataDir, path), false, r.clientConfig)
		if err != nil {
			return err
		}
		if err := manifest.Apply(&r.config.Resources[0]); err != nil {
			return err
		}
		if err := manifest.Transform(mf.InjectNamespace(operand)); err != nil {
			return err
		}
		if err := manifest.ApplyAll(); err != nil {
			return err
		}
	}
	return nil
}
