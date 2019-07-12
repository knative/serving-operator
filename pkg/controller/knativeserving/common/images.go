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
	"fmt"
	"strings"

	mf "github.com/jcrossley3/manifestival"

	"github.com/go-logr/logr"
	servingv1alpha1 "github.com/knative/serving-operator/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	caching "knative.dev/caching/pkg/apis/caching/v1alpha1"
)

var (
	// The string to be replaced by the container name
	containerNameVariable = "${NAME}"
)

func DeploymentTransform(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing, log logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Update the deployment with the new registry and tag
		if u.GetKind() == "Deployment" {
			return updateDeployment(scheme, instance, u, log)
		}
		return nil
	}
}

func ImageTransform(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing, log logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Update the image with the new registry and tag
		if u.GetAPIVersion() == "caching.internal.knative.dev/v1alpha1" && u.GetKind() == "Image" {
			return updateCachingImage(scheme, instance, u)
		}
		return nil
	}
}

func updateDeployment(scheme *runtime.Scheme, instance *servingv1alpha1.KnativeServing, u *unstructured.Unstructured, log logr.Logger) error {
	var deployment = &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, deployment)
	if err != nil {
		log.Error(err, "Error converting Unstructured to Deployment", "unstructured", u, "deployment", deployment)
		return err
	}

	registry := instance.Spec.Registry
	log.V(1).Info("Updating Deployment", "name", u.GetName(), "registry", registry)

	updateDeploymentImage(deployment, &registry, log)
	err = updateUnstructured(u, deployment, log)
	if err != nil {
		return err
	}

	log.V(1).Info("Finished conversion", "name", u.GetName(), "unstructured", u.Object)
	return nil
}

func updateUnstructured(u *unstructured.Unstructured, obj interface{}, log logr.Logger) error {
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	if err != nil {
		log.Error(err, "Error converting obj to Unstructured", "unstructured", u, "obj", obj)
		return err
	}
	u.SetUnstructuredContent(unstructuredObj)
	return nil
}

// updateDeploymentImage updates the image of the deployment with a new registry and tag
func updateDeploymentImage(deployment *appsv1.Deployment, registry *servingv1alpha1.Registry, log logr.Logger) {
	containers := deployment.Spec.Template.Spec.Containers
	for index := range containers {
		container := &containers[index]
		newImage := getNewImage(registry, container.Name)
		if newImage != "" {
			updateContainer(container, newImage, log)
		}
	}
	log.V(1).Info("Finished updating images", "name", deployment.GetName(), "containers", deployment.Spec.Template.Spec.Containers)
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

	updateImageSpec(image, &registry, log)
	err = updateUnstructured(u, image, log)
	if err != nil {
		return err
	}
	log.V(1).Info("Finished conversion", "name", u.GetName(), "unstructured", u.Object)
	return nil
}

// updateImageSpec updates the image of a with a new registry and tag
func updateImageSpec(image *caching.Image, registry *servingv1alpha1.Registry, log logr.Logger) {
	newImage := getNewImage(registry, image.Name)
	if newImage != "" {
		log.V(1).Info(fmt.Sprintf("Updating image from: %v, to: %v", image.Spec.Image, newImage))
		image.Spec.Image = newImage
	}
	log.V(1).Info("Finished updating image", "image", image.GetName())
}

func getNewImage(registry *servingv1alpha1.Registry, containerName string) string {
	overrideImage := registry.Override[containerName]
	if overrideImage != "" {
		return overrideImage
	}
	return replaceName(registry.Default, containerName)
}

func updateContainer(container *corev1.Container, newImage string, log logr.Logger) {
	log.V(1).Info(fmt.Sprintf("Updating container image from: %v, to: %v", container.Image, newImage))
	container.Image = newImage
}

func replaceName(imageTemplate string, name string) string {
	return strings.ReplaceAll(imageTemplate, containerNameVariable, name)
}
