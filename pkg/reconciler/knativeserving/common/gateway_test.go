package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis/istio/v1alpha3"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type updateGatewayTest struct {
	name           string
	gatewayName    string
	in             map[string]string
	ingressGateway servingv1alpha1.KnativeIngressGateway
	expected       map[string]string
}

var updateGatewayTests = []updateGatewayTest{
	{
		name:        "UpdatesKnativeIngressGateway",
		gatewayName: "knative-ingress-gateway",
		in: map[string]string{
			"istio": "old-istio",
		},
		ingressGateway: servingv1alpha1.KnativeIngressGateway{
			Selector: map[string]string{
				"istio": "new-istio",
			},
		},
		expected: map[string]string{
			"istio": "new-istio",
		},
	},
	{
		name:        "DoesNothingToOtherGateway",
		gatewayName: "not-knative-ingress-gateway",
		in: map[string]string{
			"istio": "old-istio",
		},
		ingressGateway: servingv1alpha1.KnativeIngressGateway{
			Selector: map[string]string{
				"istio": "new-istio",
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
	log := logf.Log.WithName(tt.name)
	logf.SetLogger(logf.ZapLogger(true))

	testScheme := runtime.NewScheme()
	unstructedGateway := makeUnstructuredGateway(t, tt, testScheme)
	instance := &servingv1alpha1.KnativeServing{
		Spec: servingv1alpha1.KnativeServingSpec{
			KnativeIngressGateway: tt.ingressGateway,
		},
	}
	gatewayTransform := GatewayTransform(testScheme, instance, log)
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

func makeUnstructuredGateway(t *testing.T, tt *updateGatewayTest, scheme *runtime.Scheme) unstructured.Unstructured {
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
