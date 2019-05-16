package openshift

import (
	"context"
	"strings"

	configv1 "github.com/openshift/api/config/v1"

	mf "github.com/jcrossley3/manifestival"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("openshift")

// Configure OpenShift if we're soaking in it
func Configure(c client.Client, scheme *runtime.Scheme) (result []mf.Transformer) {
	if t := ingress(c); t != nil {
		result = append(result, t)
	}
	if t := egress(c); t != nil {
		result = append(result, t)
	}
	if len(result) > 0 {
		// We must be on OpenShift!
		result = append(result, rbac(scheme))
	}
	return result
}

// TODO: These are addressed in master and shouldn't be required for 0.6.0
func rbac(scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) *unstructured.Unstructured {
		if u.GetKind() == "ClusterRole" && u.GetName() == "knative-serving-core" {
			role := &rbacv1.ClusterRole{}
			scheme.Convert(u, role, nil) // check for err?
		OUT:
			for i, rule := range role.Rules {
				for _, group := range rule.APIGroups {
					if group == "apps" {
						resource := "deployments/finalizers"
						log.Info("Adding RBAC", "group", group, "resource", resource)
						role.Rules[i].Resources = append(rule.Resources, resource)
						break OUT
					}
				}
			}
			// Required to open privileged ports in OpenShift
			rule := rbacv1.PolicyRule{
				Verbs:         []string{"use"},
				APIGroups:     []string{"security.openshift.io"},
				Resources:     []string{"securitycontextconstraints"},
				ResourceNames: []string{"privileged", "anyuid"},
			}
			log.Info("Adding RBAC", "rule", rule)
			role.Rules = append(role.Rules, rule)
			scheme.Convert(role, u, nil)
		}
		return u
	}
}

func ingress(c client.Client) mf.Transformer {
	ingressConfig := &configv1.Ingress{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, ingressConfig); err != nil {
		if !meta.IsNoMatchError(err) {
			log.Error(err, "Unexpected error during detection")
		}
		return nil
	}
	domain := ingressConfig.Spec.Domain
	if len(domain) == 0 {
		return nil
	}
	return func(u *unstructured.Unstructured) *unstructured.Unstructured {
		if u.GetKind() == "ConfigMap" && u.GetName() == "config-domain" {
			k, v := domain, ""
			log.Info("Setting ingress", k, v)
			unstructured.SetNestedField(u.Object, v, "data", k)
		}
		return u
	}
}

func egress(c client.Client) mf.Transformer {
	networkConfig := &configv1.Network{}
	if err := c.Get(context.TODO(), types.NamespacedName{Name: "cluster"}, networkConfig); err != nil {
		if !meta.IsNoMatchError(err) {
			log.Error(err, "Unexpected error during detection")
		}
		return nil
	}
	network := strings.Join(networkConfig.Spec.ServiceNetwork, ",")
	if len(network) == 0 {
		return nil
	}
	return func(u *unstructured.Unstructured) *unstructured.Unstructured {
		if u.GetKind() == "ConfigMap" && u.GetName() == "config-network" {
			k, v := "istio.sidecar.includeOutboundIPRanges", network
			log.Info("Setting egress", k, v)
			unstructured.SetNestedField(u.Object, v, "data", k)
		}
		return u
	}
}
