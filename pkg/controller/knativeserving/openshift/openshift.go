package openshift

import (
	"context"
	"strings"

	servingv1alpha1 "github.com/openshift-knative/knative-serving-operator/pkg/apis/serving/v1alpha1"
	"github.com/openshift-knative/knative-serving-operator/pkg/controller/knativeserving/common"
	configv1 "github.com/openshift/api/config/v1"

	mf "github.com/jcrossley3/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	maistraOperatorNamespace     = "istio-operator"
	maistraControlPlaneNamespace = "istio-system"
	caBundleConfigMapName        = "config-service-ca"
)

var (
	extension = common.Extension{
		Transformers: []mf.Transformer{ingress, egress, deploymentController},
		PreInstalls:  []common.Extender{ensureMaistra, caBundleConfigMap},
		PostInstalls: []common.Extender{ensureOpenshiftIngress},
	}
	log    = logf.Log.WithName("openshift")
	api    client.Client
	scheme *runtime.Scheme
)

// Configure OpenShift if we're soaking in it
func Configure(c client.Client, s *runtime.Scheme) (*common.Extension, error) {
	if routeExists, err := kindExists(c, "route", "route.openshift.io/v1", ""); err != nil {
		return nil, err
	} else if !routeExists {
		// Not running in OpenShift
		return nil, nil
	}

	// Register scheme
	if err := configv1.Install(s); err != nil {
		log.Error(err, "Unable to register scheme")
		return nil, err
	}

	api = c
	scheme = s
	return &extension, nil
}

// ensureMaistra ensures Maistra is installed in the cluster
func ensureMaistra(instance *servingv1alpha1.KnativeServing) error {
	namespace := instance.GetNamespace()

	log.Info("Ensuring Istio is installed in OpenShift")

	if operatorExists, err := kindExists(api, "controlplane", "istio.openshift.com/v1alpha3", namespace); err != nil {
		return err
	} else if !operatorExists {
		if istioExists, err := kindExists(api, "virtualservice", "networking.istio.io/v1alpha3", namespace); err != nil {
			return err
		} else if istioExists {
			log.Info("Maistra Operator not present but Istio CRDs already installed - assuming Istio is already setup")
			return nil
		}
		// Maistra operator not installed
		if err := installMaistraOperator(api); err != nil {
			return err
		}
	} else {
		log.Info("Maistra Operator already installed")
	}

	if controlPlaneExists, err := itemsExist(api, "controlplane", "istio.openshift.com/v1alpha3", maistraControlPlaneNamespace); err != nil {
		return err
	} else if !controlPlaneExists {
		// Maistra controlplane not installed
		if err := installMaistraControlPlane(api); err != nil {
			return err
		}
	} else {
		log.Info("Maistra ControlPlane already installed")
	}

	return nil
}

// ensureOpenshiftIngress ensures knative-openshift-ingress operator is installed
func ensureOpenshiftIngress(instance *servingv1alpha1.KnativeServing) error {
	namespace := instance.GetNamespace()
	const path = "deploy/resources/openshift-ingress/openshift-ingress-0.0.4.yaml"
	log.Info("Ensuring Knative OpenShift Ingress operator is installed")
	if manifest, err := mf.NewManifest(path, false, api); err == nil {
		transforms := []mf.Transformer{mf.InjectOwner(instance)}
		if len(namespace) > 0 {
			transforms = append(transforms, mf.InjectNamespace(namespace))
		}
		if err = manifest.Transform(transforms...); err == nil {
			err = manifest.ApplyAll()
		}
		if err != nil {
			log.Error(err, "Unable to install Maistra operator")
			return err
		}
	} else {
		log.Error(err, "Unable to create Knative OpenShift Ingress operator install manifest")
		return err
	}
	return nil
}

func installMaistraOperator(c client.Client) error {
	const path = "deploy/resources/maistra/maistra-operator-0.10.yaml"
	log.Info("Installing Maistra operator")
	if manifest, err := mf.NewManifest(path, false, c); err == nil {
		if err = ensureNamespace(c, maistraOperatorNamespace); err != nil {
			log.Error(err, "Unable to create Maistra operator namespace", "namespace", maistraOperatorNamespace)
			return err
		}
		if err = manifest.Transform(mf.InjectNamespace(maistraOperatorNamespace)); err == nil {
			err = manifest.ApplyAll()
		}
		if err != nil {
			log.Error(err, "Unable to install Maistra operator")
			return err
		}
	} else {
		log.Error(err, "Unable to create Maistra operator install manifest")
		return err
	}
	return nil
}

func installMaistraControlPlane(c client.Client) error {
	const path = "deploy/resources/maistra/maistra-controlplane-0.10.0.yaml"
	log.Info("Installing Maistra ControlPlane")
	if manifest, err := mf.NewManifest(path, false, c); err == nil {
		if err = ensureNamespace(c, maistraControlPlaneNamespace); err != nil {
			log.Error(err, "Unable to create Maistra ControlPlane namespace", "namespace", maistraControlPlaneNamespace)
			return err
		}
		if err = manifest.Transform(mf.InjectNamespace(maistraControlPlaneNamespace)); err == nil {
			err = manifest.ApplyAll()
		}
		if err != nil {
			log.Error(err, "Unable to install Maistra ControlPlane")
			return err
		}
	} else {
		log.Error(err, "Unable to create Maistra ControlPlane manifest")
		return err
	}
	return nil
}

func ingress(u *unstructured.Unstructured) error {
	if u.GetKind() == "ConfigMap" && u.GetName() == "config-domain" {
		ingressConfig := &configv1.Ingress{}
		if err := api.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, ingressConfig); err != nil {
			if !meta.IsNoMatchError(err) {
				return err
			}
			return nil
		}
		domain := ingressConfig.Spec.Domain
		if len(domain) > 0 {
			data := map[string]string{domain: ""}
			common.UpdateConfigMap(u, data, log)
		}
	}
	return nil
}

func egress(u *unstructured.Unstructured) error {
	if u.GetKind() == "ConfigMap" && u.GetName() == "config-network" {
		networkConfig := &configv1.Network{}
		if err := api.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, networkConfig); err != nil {
			if !meta.IsNoMatchError(err) {
				return err
			}
			return nil
		}
		network := strings.Join(networkConfig.Spec.ServiceNetwork, ",")
		if len(network) > 0 {
			data := map[string]string{"istio.sidecar.includeOutboundIPRanges": network}
			common.UpdateConfigMap(u, data, log)
		}
	}
	return nil
}

func deploymentController(u *unstructured.Unstructured) error {
	const volumeName = "service-ca"
	if u.GetKind() == "Deployment" && u.GetName() == "controller" {

		deploy := &appsv1.Deployment{}
		if err := scheme.Convert(u, deploy, nil); err != nil {
			return err
		}

		volumes := deploy.Spec.Template.Spec.Volumes
		for _, v := range volumes {
			if v.Name == volumeName {
				return nil
			}
		}
		deploy.Spec.Template.Spec.Volumes = append(volumes, v1.Volume{
			Name: volumeName,
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: caBundleConfigMapName,
					},
				},
			},
		})

		containers := deploy.Spec.Template.Spec.Containers
		containers[0].VolumeMounts = append(containers[0].VolumeMounts, v1.VolumeMount{
			Name:      volumeName,
			MountPath: "/var/run/secrets/kubernetes.io/servicecerts",
		})
		containers[0].Env = append(containers[0].Env, v1.EnvVar{
			Name:  "SSL_CERT_FILE",
			Value: "/var/run/secrets/kubernetes.io/servicecerts/service-ca.crt",
		})
		if err := scheme.Convert(deploy, u, nil); err != nil {
			return err
		}
	}
	return nil
}

func caBundleConfigMap(instance *servingv1alpha1.KnativeServing) error {
	cm := &v1.ConfigMap{}
	if err := api.Get(context.TODO(), types.NamespacedName{Name: caBundleConfigMapName, Namespace: instance.GetNamespace()}, cm); err != nil {
		if errors.IsNotFound(err) {
			// Define a new configmap
			cm.Name = caBundleConfigMapName
			cm.Annotations = make(map[string]string)
			cm.Annotations["service.alpha.openshift.io/inject-cabundle"] = "true"
			cm.Namespace = instance.GetNamespace()
			cm.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(instance, instance.GroupVersionKind())})
			err = api.Create(context.TODO(), cm)
			if err != nil {
				return err
			}
			// ConfigMap created successfully
			return nil
		}
		return err
	}

	return nil
}

func kindExists(c client.Client, kind string, apiVersion string, namespace string) (bool, error) {
	list := &unstructured.UnstructuredList{}
	list.SetKind(kind)
	list.SetAPIVersion(apiVersion)
	if err := c.List(context.TODO(), &client.ListOptions{Namespace: namespace}, list); err != nil {
		if meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func itemsExist(c client.Client, kind string, apiVersion string, namespace string) (bool, error) {
	list := &unstructured.UnstructuredList{}
	list.SetKind(kind)
	list.SetAPIVersion(apiVersion)
	if err := c.List(context.TODO(), &client.ListOptions{Namespace: namespace}, list); err != nil {
		return false, err
	}
	return len(list.Items) > 0, nil
}

func ensureNamespace(c client.Client, ns string) error {
	namespace := &v1.Namespace{}
	namespace.Name = ns
	if err := c.Create(context.TODO(), namespace); err != nil {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}
