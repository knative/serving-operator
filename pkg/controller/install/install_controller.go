package install

import (
	"context"
	"flag"
	"fmt"

	mf "github.com/jcrossley3/manifestival"
	servingv1alpha1 "github.com/openshift-knative/knative-serving-operator/pkg/apis/serving/v1alpha1"
	"github.com/openshift-knative/knative-serving-operator/version"

	"github.com/operator-framework/operator-sdk/pkg/predicate"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	filename = flag.String("filename", "deploy/resources",
		"The filename containing the YAML resources to apply")
	recursive = flag.Bool("recursive", false,
		"If filename is a directory, process all manifests recursively")
	installNs = flag.String("install-ns", "",
		"The namespace in which to create an Install resource, if none exist")
	log = logf.Log.WithName("controller_install")
	// Platform-specific functions to run before installation
	platformPreInstallFuncs []func(client.Client, *runtime.Scheme, string) error
	// Platform-specific functions to run after installation
	platformPostInstallFuncs []func(client.Client, *runtime.Scheme, string) error
	// Platform-specific configuration via manifestival transformations
	platformTransformFuncs []func(client.Client, *runtime.Scheme) []mf.Transformer
)

// Add creates a new Install Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	manifest, err := mf.NewManifest(*filename, *recursive, mgr.GetClient())
	if err != nil {
		return err
	}
	return add(mgr, newReconciler(mgr, manifest))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, man mf.Manifest) reconcile.Reconciler {
	return &ReconcileInstall{client: mgr.GetClient(), scheme: mgr.GetScheme(), config: man}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("install-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Install
	err = c.Watch(&source.Kind{Type: &servingv1alpha1.Install{}}, &handler.EnqueueRequestForObject{}, predicate.GenerationChangedPredicate{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileInstall{}

// ReconcileInstall reconciles a Install object
type ReconcileInstall struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client  client.Client
	scheme  *runtime.Scheme
	config  mf.Manifest
	nocache client.Client
}

func (r *ReconcileInstall) InjectConfig(config *rest.Config) error {
	c, err := client.New(config, client.Options{})
	if err != nil {
		return err
	}
	r.nocache = c
	// Make an attempt to create an Install CR, if necessary
	if len(*installNs) > 0 {
		go autoInstall(r.nocache, *installNs)
	}
	return nil
}

// Reconcile reads that state of the cluster for a Install object and makes changes based on the state read
// and what is in the Install.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileInstall) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Install")

	// Fetch the Install instance
	instance := &servingv1alpha1.Install{}
	if err := r.client.Get(context.TODO(), request.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			r.config.DeleteAll()
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}
	if len(instance.Status.Conditions) == 0 {
		instance.Status.InitializeConditions()
		r.updateStatus(instance)
	}

	stages := []func(*servingv1alpha1.Install) error{
		r.transform,
		r.install,
		r.deleteObsoleteResources,
		r.configure, // TODO: move to transform?
		r.checkDeployments,
	}

	defer r.updateStatus(instance)
	for _, stage := range stages {
		if err := stage(instance); err != nil {
			return reconcile.Result{}, err
		}
	}
	// TODO: We requeue because we can't watch for owned deployments
	// across namespaces. Our instance should really be cluster-scoped
	return reconcile.Result{Requeue: !instance.Status.IsReady()}, nil
}

// Update the status subresource
func (r *ReconcileInstall) updateStatus(instance *servingv1alpha1.Install) {

	// Account for https://github.com/kubernetes-sigs/controller-runtime/issues/406
	gvk := instance.GroupVersionKind()
	defer instance.SetGroupVersionKind(gvk)

	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		log.Error(err, "Unable to update status")
	}
}

// Transform resources as appropriate for the spec and platform
func (r *ReconcileInstall) transform(instance *servingv1alpha1.Install) error {
	fns := []mf.Transformer{mf.InjectOwner(instance)}
	if len(instance.Spec.Namespace) > 0 {
		fns = append(fns, mf.InjectNamespace(instance.Spec.Namespace))
	}
	for _, f := range platformTransformFuncs {
		fns = append(fns, f(r.client, r.scheme)...)
	}
	r.config.Transform(fns...)
	return nil
}

// Apply the embedded resources
func (r *ReconcileInstall) install(instance *servingv1alpha1.Install) error {
	if instance.Status.Version == version.Version {
		// we've already successfully applied our YAML
		return nil
	}
	// Ensure needed prerequisites are installed
	for _, f := range platformPreInstallFuncs {
		if err := f(r.client, r.scheme, instance.Spec.Namespace); err != nil {
			return err
		}
	}
	if err := r.config.ApplyAll(); err != nil {
		instance.Status.MarkInstallFailed(err.Error())
		return err
	}
	// Do any post-installation tasks
	for _, f := range platformPostInstallFuncs {
		if err := f(r.client, r.scheme, instance.Spec.Namespace); err != nil {
			return err
		}
	}
	// Update status
	instance.Status.Resources = r.config.Resources
	instance.Status.Version = version.Version
	instance.Status.MarkInstallSucceeded()
	return nil
}

// Check for all deployments available
// TODO: verify that all the Deployments in the config are available
func (r *ReconcileInstall) checkDeployments(instance *servingv1alpha1.Install) error {
	deployments := &v1.DeploymentList{}
	controller := r.config.Find("apps/v1", "Deployment", "controller")
	err := r.nocache.List(context.TODO(), &client.ListOptions{Namespace: controller.GetNamespace()}, deployments)
	if err != nil {
		log.Error(err, "Unable to list Deployments")
		return err
	}
	available := func(d v1.Deployment) bool {
		for _, c := range d.Status.Conditions {
			if c.Type == v1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
				return true
			}
		}
		return false
	}
	allAvailable := func() bool {
		for _, deploy := range deployments.Items {
			if !available(deploy) {
				return false
			}
		}
		return true
	}
	// TODO: Instead of a count, verify specific Deployments
	if len(deployments.Items) >= 4 && allAvailable() {
		instance.Status.MarkDeploymentsAvailable()
	} else {
		instance.Status.MarkDeploymentsNotReady()
	}
	return nil
}

// Set ConfigMap values from Install spec
func (r *ReconcileInstall) configure(instance *servingv1alpha1.Install) error {
	for suffix, config := range instance.Spec.Config {
		name := "config-" + suffix
		cm, err := r.config.Get(r.config.Find("v1", "ConfigMap", name))
		if err != nil {
			return err
		}
		if cm == nil {
			log.Error(fmt.Errorf("ConfigMap '%s' not found", name), "Invalid Install spec")
			continue
		}
		if err := r.updateConfigMap(cm, config); err != nil {
			return err
		}
	}
	return nil
}

// Set some data in a configmap, only overwriting common keys
func (r *ReconcileInstall) updateConfigMap(cm *unstructured.Unstructured, data map[string]string) error {
	for k, v := range data {
		message := []interface{}{"map", cm.GetName(), k, v}
		if x, found, _ := unstructured.NestedString(cm.Object, "data", k); found {
			if v == x {
				continue
			}
			message = append(message, "previous", x)
		}
		log.Info("Setting", message...)
		unstructured.SetNestedField(cm.Object, v, "data", k)
	}
	return r.config.Apply(cm)
}

// Delete obsolete istio-system resources, if any
func (r *ReconcileInstall) deleteObsoleteResources(instance *servingv1alpha1.Install) error {
	resource := &unstructured.Unstructured{}
	resource.SetNamespace("istio-system")
	resource.SetName("knative-ingressgateway")
	resource.SetAPIVersion("v1")
	resource.SetKind("Service")
	if err := r.config.Delete(resource); err != nil {
		return err
	}
	resource.SetAPIVersion("apps/v1")
	resource.SetKind("Deployment")
	if err := r.config.Delete(resource); err != nil {
		return err
	}
	resource.SetAPIVersion("autoscaling/v1")
	resource.SetKind("HorizontalPodAutoscaler")
	return r.config.Delete(resource)
}

// This may or may not be a good idea
func autoInstall(c client.Client, ns string) (err error) {
	const path = "deploy/crds/serving_v1alpha1_install_cr.yaml"
	log.Info("Automatic Install requested", "namespace", ns)
	installList := &servingv1alpha1.InstallList{}
	err = c.List(context.TODO(), &client.ListOptions{Namespace: ns}, installList)
	if err != nil {
		log.Error(err, "Unable to list Installs")
		return err
	}
	if len(installList.Items) == 0 {
		if manifest, err := mf.NewManifest(path, false, c); err == nil {
			if err = manifest.Transform(mf.InjectNamespace(ns)).ApplyAll(); err != nil {
				log.Error(err, "Unable to create Install")
			}
		} else {
			log.Error(err, "Unable to create Install manifest")
		}
	} else {
		log.Info("Install found", "name", installList.Items[0].Name)
	}
	return err
}
