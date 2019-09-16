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
package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis/istio/v1alpha3"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
)

type updateGatewayTest struct {
	name                  string
	gatewayName           string
	in                    map[string]string
	knativeIngressGateway servingv1alpha1.IstioGatewayOverride
	clusterLocalGateway   servingv1alpha1.IstioGatewayOverride
	expected              map[string]string
}

var updateGatewayTests = []updateGatewayTest{
	{
		name:        "UpdatesKnativeIngressGateway",
		gatewayName: "knative-ingress-gateway",
		in: map[string]string{
			"istio": "old-istio",
		},
		knativeIngressGateway: servingv1alpha1.IstioGatewayOverride{
			Selector: map[string]string{
				"istio": "knative-ingress",
			},
		},
		clusterLocalGateway: servingv1alpha1.IstioGatewayOverride{
			Selector: map[string]string{
				"istio": "cluster-local",
			},
		},
		expected: map[string]string{
			"istio": "knative-ingress",
		},
	},
	{
		name:        "UpdatesClusterLocalGateway",
		gatewayName: "cluster-local-gateway",
		in: map[string]string{
			"istio": "old-istio",
		},
		knativeIngressGateway: servingv1alpha1.IstioGatewayOverride{
			Selector: map[string]string{
				"istio": "knative-ingress",
			},
		},
		clusterLocalGateway: servingv1alpha1.IstioGatewayOverride{
			Selector: map[string]string{
				"istio": "cluster-local",
			},
		},
		expected: map[string]string{
			"istio": "cluster-local",
		},
	},
	{
		name:        "DoesNothingToOtherGateway",
		gatewayName: "not-knative-ingress-gateway",
		in: map[string]string{
			"istio": "old-istio",
		},
		knativeIngressGateway: servingv1alpha1.IstioGatewayOverride{
			Selector: map[string]string{
				"istio": "knative-ingress",
			},
		},
		clusterLocalGateway: servingv1alpha1.IstioGatewayOverride{
			Selector: map[string]string{
				"istio": "cluster-local",
			},
		},
		expected: map[string]string{
			"istio": "old-istio",
		},
	},
}

func TestGatewayTransform(t *testing.T) {
	for _, tt := range updateGatewayTests {
		t.Run(tt.name, func(t *testing.T) {
			runGatewayTransformTest(t, &tt)
		})
	}
}
func runGatewayTransformTest(t *testing.T, tt *updateGatewayTest) {
	unstructedGateway := makeUnstructuredGateway(t, tt)
	instance := &servingv1alpha1.KnativeServing{
		Spec: servingv1alpha1.KnativeServingSpec{
			KnativeIngressGateway: tt.knativeIngressGateway,
			ClusterLocalGateway:   tt.clusterLocalGateway,
		},
	}
	gatewayTransform := GatewayTransform(instance, log)
	gatewayTransform(&unstructedGateway)
	validateUnstructedGatewayChanged(t, tt, &unstructedGateway)
}

func validateUnstructedGatewayChanged(t *testing.T, tt *updateGatewayTest, u *unstructured.Unstructured) {
	var gateway = &v1alpha3.Gateway{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, gateway)
	assertEqual(t, err, nil)
	for expectedKey, expectedValue := range tt.expected {
		assertEqual(t, gateway.Spec.Selector[expectedKey], expectedValue)
	}
}

func makeUnstructuredGateway(t *testing.T, tt *updateGatewayTest) unstructured.Unstructured {
	gateway := v1alpha3.Gateway{
		Spec: v1alpha3.GatewaySpec{
			Selector: tt.in,
		},
	}
	gateway.APIVersion = "networking.istio.io/v1alpha3"
	gateway.Kind = "Gateway"
	gateway.Name = tt.gatewayName
	unstructuredDeployment, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&gateway)
	if err != nil {
		t.Fatalf("Could not create unstructured deployment object: %v, err: %v", unstructuredDeployment, err)
	}
	return unstructured.Unstructured{
		Object: unstructuredDeployment,
	}
}
