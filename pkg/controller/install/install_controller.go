package install

import (
	"context"
	"flag"
	"os"

	mf "github.com/jcrossley3/manifestival"
	servingv1alpha1 "github.com/openshift-knative/knative-serving-operator/pkg/apis/serving/v1alpha1"
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
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileInstall{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
		config: mf.NewYamlManifest(*filename, *recursive, mgr.GetConfig())}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
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

	// Auto-create Install
	if *autoinstall {
		ns, _ := k8sutil.GetWatchNamespace()
		go autoInstall(mgr.GetClient(), ns)
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
	if err := r.install(instance); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.deleteObsoleteResources(); err != nil {
		return reconcile.Result{}, err
	}
	if err := r.checkForMinikube(); err != nil {
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

// Apply the embedded resources
func (r *ReconcileInstall) install(instance *servingv1alpha1.Install) error {
	if instance.Status.Version == getResourceVersion() {
		// we've already successfully applied our YAML
		return nil
	}
	// Filter resources as appropriate
	filters := []mf.FilterFn{mf.ByOwner(instance)}
	switch {
	case *olm:
		filters = append(filters, mf.ByOLM, mf.ByNamespace(instance.GetNamespace()))
	case len(*namespace) > 0:
		filters = append(filters, mf.ByNamespace(*namespace))
	}
	// Apply the resources in the YAML file
	if err := r.config.Filter(filters...).ApplyAll(); err != nil {
		return err
	}
	// Update status
	instance.Status.Resources = r.config.ResourceNames()
	instance.Status.Version = getResourceVersion()
	if err := r.client.Status().Update(context.TODO(), instance); err != nil {
		return err
	}
	return nil
}

// Delete obsolete istio-system resources, if any
func (r *ReconcileInstall) deleteObsoleteResources() error {
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
func (r *ReconcileInstall) checkForMinikube() error {
	node := &v1.Node{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "minikube"}, node)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // not running on minikube!
		}
		return err
	}
	cm := &v1.ConfigMap{}
	u := r.config.Find("v1", "ConfigMap", "config-network") // 4 the ns
	key := types.NamespacedName{Namespace: u.GetNamespace(), Name: u.GetName()}
	if err := r.client.Get(context.TODO(), key, cm); err != nil {
		return err
	}
	cm.Data["istio.sidecar.includeOutboundIPRanges"] = "10.0.0.1/24"
	return r.client.Update(context.TODO(), cm)
}

func getResourceVersion() string {
	v, found := os.LookupEnv("RESOURCE_VERSION")
	if !found {
		return "UNKNOWN"
	}
	return v
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
