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
	mf "github.com/jcrossley3/manifestival"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("extensions")

type Platforms []func(kubernetes.Interface) (mf.Transformer, error)

func (platforms Platforms) Transformers(kubeClientSet kubernetes.Interface, scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing) ([]mf.Transformer, error) {
	log.V(1).Info("Transforming", "instance", instance)
	result := []mf.Transformer{
		mf.InjectOwner(instance),
		mf.InjectNamespace(instance.GetNamespace()),
		ConfigMapTransform(instance, log),
		DeploymentTransform(scheme, instance, log),
		ImageTransform(scheme, instance, log),
		GatewayTransform(scheme, instance, log),
	}
	for _, fn := range platforms {
		transformer, err := fn(kubeClientSet)
		if err != nil {
			return result, err
		}
		if transformer != nil {
			result = append(result, transformer)
		}
	}
	return result, nil
}
