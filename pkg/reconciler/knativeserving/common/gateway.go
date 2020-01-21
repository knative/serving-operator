package common

/*
Copyright 2020 The Knative Authors
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

import (
	"fmt"

	mf "github.com/jcrossley3/manifestival"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
)

func GatewayTransform(instance *servingv1alpha1.KnativeServing, log *zap.SugaredLogger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Update the deployment with the new registry and tag
		if u.GetAPIVersion() == "networking.istio.io/v1alpha3" && u.GetKind() == "Gateway" {
			if u.GetName() == "knative-ingress-gateway" {
				return updateKnativeIngressGateway(instance.Spec.KnativeIngressGateway, u)
			}
			if u.GetName() == "cluster-local-gateway" {
				return updateKnativeIngressGateway(instance.Spec.ClusterLocalGateway, u)
			}
		}
		return nil
	}
}

func updateKnativeIngressGateway(gatewayOverrides servingv1alpha1.IstioGatewayOverride, u *unstructured.Unstructured) error {
	if len(gatewayOverrides.Selector) > 0 {
		log.Debugw("Updating Selector Section for Gateway", "name", u.GetName(), "gatewayOverrides", gatewayOverrides)
		unstructured.SetNestedStringMap(u.Object, gatewayOverrides.Selector, "spec", "selector")
		log.Debugw("Finished Selector Section conversion for Gateway", "name", u.GetName(), "unstructured", u.Object)
	}
	if len(gatewayOverrides.Servers) > 0 {
		log.Debugw("Updating Servers section for Gateway", "name", u.GetName(), "gatewayOverrides", gatewayOverrides)

		newReferences := make([]interface{}, 0, len(gatewayOverrides.Servers))
		for _, reference := range gatewayOverrides.Servers {
			out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&reference)
			if err != nil {
				utilruntime.HandleError(fmt.Errorf("unable to convert Server Reference: %v", err))
				continue
			}
			newReferences = append(newReferences, out)
		}
		unstructured.SetNestedSlice(u.Object, newReferences, "spec", "servers")
		log.Debugw("Finished Servers Section conversion for Gateway", "name", u.GetName(), "unstructured", u.Object)
	}
	return nil
}
