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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("common")

type Platforms []func(kubernetes.Interface, dynamic.Interface) (*Extension, error)
type Extender func(*servingv1alpha1.KnativeServing) error
type Extensions []Extension
type Extension struct {
	Transformers []mf.Transformer
	PreInstalls  []Extender
	PostInstalls []Extender
}

func (platforms Platforms) Extend(kubeClientSet kubernetes.Interface, dynamicClientSet dynamic.Interface) (result Extensions, err error) {
	for _, fn := range platforms {
		ext, err := fn(kubeClientSet, dynamicClientSet)
		if err != nil {
			return result, err
		}
		if ext != nil {
			result = append(result, *ext)
		}
	}
	return
}

func (exts Extensions) Transform(instance *servingv1alpha1.KnativeServing) []mf.Transformer {
	log.V(1).Info("Transforming", "instance", instance)
	result := []mf.Transformer{
		mf.InjectOwner(instance),
		mf.InjectNamespace(instance.GetNamespace()),
		ConfigMapTransform(instance, log),
		DeploymentTransform(instance, log),
		ImageTransform(instance, log),
		GatewayTransform(instance, log),
	}
	for _, extension := range exts {
		result = append(result, extension.Transformers...)
	}
	return result
}

func (exts Extensions) PreInstall(instance *servingv1alpha1.KnativeServing) error {
	for _, extension := range exts {
		for _, f := range extension.PreInstalls {
			if err := f(instance); err != nil {
				return err
			}
		}
	}
	return nil
}

func (exts Extensions) PostInstall(instance *servingv1alpha1.KnativeServing) error {
	for _, extension := range exts {
		for _, f := range extension.PostInstalls {
			if err := f(instance); err != nil {
				return err
			}
		}
	}
	return nil
}
