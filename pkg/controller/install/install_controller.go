package install

import (
	"context"
	"flag"

	mf "github.com/jcrossley3/manifestival"
	servingv1alpha1 "github.com/openshift-knative/knative-serving-operator/pkg/apis/serving/v1alpha1"
	"github.com/openshift-knative/knative-serving-operator/pkg/controller/install/common"
	"github.com/openshift-knative/knative-serving-operator/version"

	"github.com/operator-framework/operator-sdk/pkg/predicate"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	log = logf.Log.WithName("controller_install")
	// Platform-specific functions to run before installation
	platformPreInstallFuncs []func(client.Client, *runtime.Scheme, *servingv1alpha1.Install) error
	// Platform-specific functions to run after installation
	platformPostInstallFuncs []func(client.Client, *runtime.Scheme, *servingv1alpha1.Install) error
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

	// Watch child deployments for availability
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &servingv1alpha1.Install{},
	})
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
	client client.Client
	scheme *runtime.Scheme
	config mf.Manifest
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

	stages := []func(*servingv1alpha1.Install) error{
		r.initStatus,
		r.transform,
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
func (r *ReconcileInstall) initStatus(instance *servingv1alpha1.Install) error {
	if len(instance.Status.Conditions) == 0 {
		instance.Status.InitializeConditions()
		if err := r.updateStatus(instance); err != nil {
			return err
		}
	}
	return nil
}

// Update the status subresource
func (r *ReconcileInstall) updateStatus(instance *servingv1alpha1.Install) error {

	// Account for https://github.com/kubernetes-sigs/controller-runtime/issues/406
	gvk := instance.GroupVersionKind()
	defer instance.SetGroupVersionKind(gvk)

	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		return err
	}
	return nil
}

// Transform resources as appropriate for the spec and platform
func (r *ReconcileInstall) transform(instance *servingv1alpha1.Install) error {
	fns := []mf.Transformer{
		mf.InjectOwner(instance),
		mf.InjectNamespace(instance.GetNamespace()),
	}
	for _, f := range platformTransformFuncs {
		fns = append(fns, f(r.client, r.scheme)...)
	}
	// Let any config in instance override everything else
	fns = append(fns, configure(instance))

	r.config.Transform(fns...)
	return nil
}

// Apply the embedded resources
func (r *ReconcileInstall) install(instance *servingv1alpha1.Install) error {
	if instance.Status.IsDeploying() {
		return nil
	}
	defer r.updateStatus(instance)
	// Ensure needed prerequisites are installed
	for _, f := range platformPreInstallFuncs {
		if err := f(r.client, r.scheme, instance); err != nil {
			return err
		}
	}
	if err := r.config.ApplyAll(); err != nil {
		instance.Status.MarkInstallFailed(err.Error())
		return err
	}
	// Do any post-installation tasks
	for _, f := range platformPostInstallFuncs {
		if err := f(r.client, r.scheme, instance); err != nil {
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
	defer r.updateStatus(instance)
	deployments := &appsv1.DeploymentList{}
	controller := r.config.Find("apps/v1", "Deployment", "controller")
	err := r.client.List(context.TODO(), &client.ListOptions{Namespace: controller.GetNamespace()}, deployments)
	if err != nil {
		log.Error(err, "Unable to list Deployments")
		return err
	}
	available := func(d appsv1.Deployment) bool {
		for _, c := range d.Status.Conditions {
			if c.Type == appsv1.DeploymentAvailable && c.Status == v1.ConditionTrue {
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

// Set ConfigMap values from Install spec
func configure(instance *servingv1alpha1.Install) mf.Transformer {
	return func(u *unstructured.Unstructured) *unstructured.Unstructured {
		if u.GetKind() == "ConfigMap" {
			if data, ok := instance.Spec.Config[u.GetName()[7:]]; ok {
				common.UpdateConfigMap(u, data, log)
			}
		}
		return u
	}
}
