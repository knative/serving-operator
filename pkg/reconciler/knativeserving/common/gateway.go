package common

import (
	"github.com/go-logr/logr"
	mf "github.com/jcrossley3/manifestival"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
)

func GatewayTransform(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing, log logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Update the deployment with the new registry and tag
		if u.GetAPIVersion() == "networking.istio.io/v1alpha3" && u.GetKind() == "Gateway" {
			if u.GetName() == "knative-ingress-gateway" {
				return updateKnativeIngressGateway(scheme, instance.Spec.KnativeIngressGateway, u)
			}
			if u.GetName() == "cluster-local-gateway" {
				return updateKnativeIngressGateway(scheme, instance.Spec.ClusterLocalGateway, u)
			}
		}
		return nil
	}
}

func updateKnativeIngressGateway(scheme *runtime.Scheme, gatewayOverrides servingv1alpha1.IstioGatewayOverride, u *unstructured.Unstructured) error {
	if len(gatewayOverrides.Selector) > 0 {
		log.V(1).Info("Updating Gateway", "name", u.GetName(), "gatewayOverrides", gatewayOverrides)
		unstructured.SetNestedStringMap(u.Object, gatewayOverrides.Selector, "spec", "selector")
		log.V(1).Info("Finished conversion", "name", u.GetName(), "unstructured", u.Object)
	}
	return nil
}
