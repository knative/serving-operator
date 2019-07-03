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
	servingv1alpha1 "github.com/knative/serving-operator/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	caching "knative.dev/caching/pkg/apis/caching/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("common")

type Platforms []func(client.Client, *runtime.Scheme) (*Extension, error)
type Extender func(*servingv1alpha1.KnativeServing) error
type Extensions []Extension
type Extension struct {
	Transformers []mf.Transformer
	PreInstalls  []Extender
	PostInstalls []Extender
}

func (platforms Platforms) Extend(c client.Client, scheme *runtime.Scheme) (result Extensions, err error) {
	for _, fn := range platforms {
		ext, err := fn(c, scheme)
		if err != nil {
			return result, err
		}
		if ext != nil {
			result = append(result, *ext)
		}
	}
	return
}

func (exts Extensions) Transform(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing) []mf.Transformer {
	log.V(1).Info("Transforming", "instance", instance)
	result := []mf.Transformer{
		mf.InjectOwner(instance),
		mf.InjectNamespace(instance.GetNamespace()),
	}
	for _, extension := range exts {
		result = append(result, extension.Transformers...)
	}
	return append(result, func(u *unstructured.Unstructured) error {
		// Update the deployment with the new registry and tag
		if u.GetAPIVersion() == "caching.internal.knative.dev/v1alpha1" && u.GetKind() == "Image" {
			updateCachingImage(scheme, instance, u)
		}
		if u.GetKind() == "Deployment" {
			updateDeployment(scheme, instance, u)
		}
		// Let any config in instance override everything else
		if u.GetKind() == "ConfigMap" {
			if data, ok := instance.Spec.Config[u.GetName()[len(`config-`):]]; ok {
				UpdateConfigMap(u, data, log)
			}
		}
		return nil
	})
}

func updateCachingImage(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing, u *unstructured.Unstructured) error {
	var image = &caching.Image{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, image)
	if err != nil {
		log.Error(err, "Error converting Unstructured to Image", "unstructured", u, "image", image)
		return err
	}

	registry := instance.Spec.Registry
	log.V(1).Info("Updating Image", "name", u.GetName(), "registry", registry)

	UpdateImageSpec(image, &registry, log)
	scheme.Convert(image, u, nil)
	log.V(1).Info("Finished conversion", "name", u.GetName(), "unstructured", u.Object)
	return nil
}

func updateDeployment(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing, u *unstructured.Unstructured) error {
	var deployment = &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, deployment)
	if err != nil {
		log.Error(err, "Error converting Unstructured to Deployment", "unstructured", u, "deployment", deployment)
		return err
	}

	registry := instance.Spec.Registry
	log.V(1).Info("Updating Deployment", "name", u.GetName(), "registry", registry)

	UpdateDeploymentImage(deployment, &registry, log)
	scheme.Convert(deployment, u, nil)
	log.V(1).Info("Finished conversion", "name", u.GetName(), "unstructured", u.Object)
	return nil
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
