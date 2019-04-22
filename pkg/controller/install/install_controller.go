package install

import (
	"context"
	"flag"
	"fmt"
	"strings"

	mf "github.com/jcrossley3/manifestival"
	servingv1alpha1 "github.com/openshift-knative/knative-serving-operator/pkg/apis/serving/v1alpha1"
	"github.com/openshift-knative/knative-serving-operator/version"
	configv1 "github.com/openshift/api/config/v1"

	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	autoinstall = flag.Bool("install", false,
		"Automatically creates an Install resource if none exist")
	olm = flag.Bool("olm", false,
		"Ignores resources managed by the Operator Lifecycle Manager")
	namespace = flag.String("namespace", "",
		"Overrides the hard-coded namespace references in the manifest")
	log = logf.Log.WithName("controller_install")
)

// Add creates a new Install Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	manifest, err := mf.NewYamlManifest(*filename, *recursive, mgr.GetClient())
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
	// Register scheme
	if err := configv1.Install(mgr.GetScheme()); err != nil {
		log.Error(err, "Unable to register scheme")
	}

	// Create a new controller
	c, err := controller.New("install-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Install
	err = c.Watch(&source.Kind{Type: &servingv1alpha1.Install{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Make an attempt to auto-create an Install CR
	if *autoinstall {
		ns, _ := k8sutil.GetWatchNamespace()
		c, _ := client.New(mgr.GetConfig(), client.Options{})
		go autoInstall(c, ns)
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
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			r.config.DeleteAll()
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	stages := []func(*servingv1alpha1.Install) error{
		r.install,
		r.deleteObsoleteResources,
		r.checkForMinikube,
		r.updateServiceNetwork,
		r.updateDomain,
		r.configure,
	}

	for _, stage := range stages {
		if err := stage(instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// Apply the embedded resources
func (r *ReconcileInstall) install(instance *servingv1alpha1.Install) error {
	// Filter resources as appropriate
	filters := []mf.FilterFn{mf.ByOwner(instance)}
	switch {
	case *olm:
		sa, err := k8sutil.GetOperatorName()
		if err != nil {
			return err
		}
		filters = append(filters,
			mf.ByOLM,
			mf.ByNamespace(instance.GetNamespace()),
			mf.ByServiceAccount(sa))
	case len(*namespace) > 0:
		filters = append(filters, mf.ByNamespace(*namespace))
	}
	r.config.Filter(filters...)

	if instance.Status.Version == version.Version {
		// we've already successfully applied our YAML
		return nil
	}
	// Apply the resources in the YAML file
	if err := r.config.ApplyAll(); err != nil {
		return err
	}

	// Update status
	instance.Status.Resources = r.config.ResourceNames()
	instance.Status.Version = version.Version
	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		return err
	}
	return nil
}

// Set ConfigMap values from Install spec
func (r *ReconcileInstall) configure(instance *servingv1alpha1.Install) error {
	for suffix, config := range instance.Spec.Config {
		name := "config-" + suffix
		cm := r.config.Find("v1", "ConfigMap", name)
		if cm == nil {
			log.Error(fmt.Errorf("ConfigMap '%s' not found", name), "Invalid Install spec")
			continue
		}
		for k, v := range config {
			unstructured.SetNestedField(cm.Object, v, "data", k)
		}
		if err := r.config.Apply(cm); err != nil {
			return err
		}
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

// Configure minikube if we're soaking in it
func (r *ReconcileInstall) checkForMinikube(instance *servingv1alpha1.Install) error {
	node := &v1.Node{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "minikube"}, node)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // not running on minikube!
		}
		return err
	}

	cm, err := r.config.Get(r.config.Find("v1", "ConfigMap", "config-network"))
	if err != nil {
		return err
	}
	if cm == nil {
		log.Error(err, "Missing ConfigMap", "name", "config-network")
		return nil // no sense in trying if the CM is gone
	}
	const k, v = "istio.sidecar.includeOutboundIPRanges", "10.0.0.1/24"
	if _, found, _ := unstructured.NestedString(cm.Object, "data", k); found {
		log.V(1).Info("Detected minikube; egress already configured", k, v)
		return nil // already set
	}
	log.Info("Detected minikube; configuring egress", k, v)
	unstructured.SetNestedField(cm.Object, v, "data", k)
	return r.client.Update(context.TODO(), cm)

}

// Get Service Network from cluster resource
func (r *ReconcileInstall) getServiceNetwork() string {
	networkConfig := &configv1.Network{}
	serviceNetwork := ""
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, networkConfig); err != nil {
		log.V(1).Info("OpenShift Network Config is not available.")
	} else if len(networkConfig.Spec.ServiceNetwork) > 0 {
		serviceNetwork = strings.Join(networkConfig.Spec.ServiceNetwork, ",")
		log.Info("OpenShift Network Config is available", "Service Network", serviceNetwork)
	}
	return serviceNetwork
}

func (r *ReconcileInstall) getDomain() string {
	ingressConfig := &configv1.Ingress{}
	domain := ""
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, ingressConfig); err != nil {
		log.V(1).Info("OpenShift Ingress Config is not available.")
	} else {
		domain = ingressConfig.Spec.Domain
		log.Info("OpenShift Ingress Config is available", "Domain", domain)
	}

	return domain
}

// Set domain in the Config Map
func (r *ReconcileInstall) updateDomain(instance *servingv1alpha1.Install) error {

	// retrieve domain for configuring for ingress traffic
	domain := r.getDomain()

	// If domain is available, update config-domain config map
	if len(domain) > 0 {

		cm := &v1.ConfigMap{}
		u := r.config.Find("v1", "ConfigMap", "config-domain")
		key := types.NamespacedName{Namespace: u.GetNamespace(), Name: u.GetName()}
		if err := r.client.Get(context.TODO(), key, cm); err != nil {
			return err
		}
		cm.Data[domain] = ""
		return r.client.Update(context.TODO(), cm)
	}

	return nil
}

// Set istio.sidecar.includeOutboundIPRanges property with service network
func (r *ReconcileInstall) updateServiceNetwork(instance *servingv1alpha1.Install) error {

	// retrieve service networks for configuring egress traffic
	serviceNetwork := r.getServiceNetwork()

	// If service network is available, update config-network config map
	if len(serviceNetwork) > 0 {

		cm := &v1.ConfigMap{}
		u := r.config.Find("v1", "ConfigMap", "config-network")
		key := types.NamespacedName{Namespace: u.GetNamespace(), Name: u.GetName()}
		if err := r.client.Get(context.TODO(), key, cm); err != nil {
			return err
		}
		cm.Data["istio.sidecar.includeOutboundIPRanges"] = serviceNetwork
		return r.client.Update(context.TODO(), cm)

	}

	return nil
}

func autoInstall(c client.Client, ns string) error {
	installList := &servingv1alpha1.InstallList{}
	err := c.List(context.TODO(), &client.ListOptions{Namespace: ns}, installList)
	if err != nil {
		log.Error(err, "Unable to list Installs")
		return err
	}
	if len(installList.Items) == 0 {
		install := &servingv1alpha1.Install{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "auto-install",
				Namespace: ns,
			},
		}
		err = c.Create(context.TODO(), install)
		if err != nil {
			log.Error(err, "Unable to create Install")
		}
	}
	return nil
}
