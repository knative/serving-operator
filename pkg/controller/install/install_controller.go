package install

import (
	"context"
	"flag"
	"os"
	"strings"

	"github.com/jcrossley3/manifestival/yaml"

	kscheme "k8s.io/client-go/kubernetes/scheme"

	servingv1alpha1 "github.com/openshift-knative/knative-serving-operator/pkg/apis/serving/v1alpha1"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

const (
	// Namespace of Knative Serving
	knativeServingNamespace = "knative-serving"

	// ConfigMap name for config-network
	networkConfigMapName = "config-network"

	// ConfigMap name for config-domain
	domainConfigMapName = "config-domain"

	// Key name for example config in config map
	exampleKey = "_example"

	// cluster object name to retrieve network/domain info
	clusterObjectName = "cluster"

	// istio.sidecar.includeOutboundIPRanges property name
	istioSideCarIncludeOutboundIPRangesProp = "istio.sidecar.includeOutboundIPRanges"
)

var (
	filename = flag.String("filename", "deploy/resources",
		"The filename containing the YAML resources to apply")
	autoinstall = flag.Bool("install", false,
		"Automatically creates an Install resource if none exist")
	log = logf.Log.WithName("controller_install")

	scheme *runtime.Scheme
)

func init() {
	// register openshift api scheme
	scheme = kscheme.Scheme
	if err := configv1.Install(scheme); err != nil {
		panic(err)
	}
}

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
		config: yaml.NewYamlManifest(*filename, mgr.GetConfig())}
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
	config *yaml.YamlManifest
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
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			r.config.Delete()
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	if instance.Status.Resources != nil {
		// we've already successfully applied our YAML
		return reconcile.Result{}, nil
	}

	// retrieve service networks for configuring egress traffic
	networkConfig := &configv1.Network{}
	serviceNetwork := ""
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: clusterObjectName}, networkConfig); err != nil {
		reqLogger.Info("Network Config is not available.")
	} else if len(networkConfig.Spec.ServiceNetwork) > 0 {
		serviceNetwork = strings.Join(networkConfig.Spec.ServiceNetwork, ",")
		reqLogger.Info("Network Config is available", "Service Network", serviceNetwork)
	}

	// retrieve domain for configuring for ingress traffic
	ingressConfig := &configv1.Ingress{}
	domain := ""
	if err := r.client.Get(context.TODO(), types.NamespacedName{Name: clusterObjectName}, ingressConfig); err != nil {
		reqLogger.Info("Ingress Config is not available.")
	} else {
		domain = ingressConfig.Spec.Domain
		reqLogger.Info("Ingress Config is available", "Domain", domain)
	}

	// Apply the resources in the YAML file
	err = r.config.Apply(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	// Update status
	instance.Status.Resources = r.config.ResourceNames()
	instance.Status.Version = getResourceVersion()
	err = r.client.Status().Update(context.TODO(), instance)
	if err != nil {
		reqLogger.Error(err, "Failed to update status")
		return reconcile.Result{}, err
	}

	// If domain is available, update config-domain config map
	if len(domain) > 0 {
		configMap := &corev1.ConfigMap{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: knativeServingNamespace,
			Name: domainConfigMapName}, configMap); err != nil {
			reqLogger.Error(err, "Failed to get configmap for config-domain")
		} else {
			delete(configMap.Data, exampleKey)
			configMap.Data[domain] = ""
			if err := r.client.Update(context.TODO(), configMap); err != nil {
				reqLogger.Error(err, "Failed to update configmap for config-domain")
			}
		}
	}

	// If service network is available, update config-network config map
	if len(serviceNetwork) > 0 {
		configMap := &corev1.ConfigMap{}
		if err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: knativeServingNamespace,
			Name: networkConfigMapName}, configMap); err != nil {
			reqLogger.Error(err, "Failed to get configmap for config-network")
		} else {
			delete(configMap.Data, exampleKey)
			configMap.Data[istioSideCarIncludeOutboundIPRangesProp] = serviceNetwork
			if err := r.client.Update(context.TODO(), configMap); err != nil {
				reqLogger.Error(err, "Failed to update configmap for config-network")
			}
		}
	}

	return reconcile.Result{}, nil
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
