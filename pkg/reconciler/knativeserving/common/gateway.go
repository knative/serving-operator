package common

import (
	"github.com/go-logr/logr"
	mf "github.com/jcrossley3/manifestival"
	servingv1alpha1 "github.com/knative/serving-operator/pkg/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func GatewayTransform(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing, log logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Update the deployment with the new registry and tag
		if u.GetAPIVersion() == "networking.istio.io/v1alpha3" && u.GetKind() == "Gateway" && u.GetName() == "knative-ingress-gateway" {
			return updateKnativeIngressGateway(scheme, instance, u)
		}
		return nil
	}
}

func updateKnativeIngressGateway(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing, u *unstructured.Unstructured) error {
	gatewayOverrides := instance.Spec.KnativeIngressGateway
	if len(gatewayOverrides.Selector) > 0 {
		log.V(1).Info("Updating Gateway", "name", u.GetName(), "gatewayOverrides", gatewayOverrides)
		unstructured.SetNestedStringMap(u.Object, gatewayOverrides.Selector, "spec", "selector")
		log.V(1).Info("Finished conversion", "name", u.GetName(), "unstructured", u.Object)
	}
	return nil
}
