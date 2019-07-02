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

	"github.com/go-logr/logr"
	servingv1alpha1 "github.com/knative/serving-operator/pkg/apis/serving/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	caching "knative.dev/caching/pkg/apis/caching/v1alpha1"
)

var (
	// The string to be replaced by the container name
	containerNameVariable = "${NAME}"
)

// UpdateDeploymentImage updates the image of the deployment with a new registry and tag
func UpdateDeploymentImage(deployment *appsv1.Deployment, registry *servingv1alpha1.Registry, log logr.Logger) {
	containers := deployment.Spec.Template.Spec.Containers
	for index := range containers {
		container := &containers[index]
		newImage := getNewImage(registry, container.Name)
		if newImage != "" {
			updateContainer(container, newImage, log)
		}
	}
	log.V(1).Info("Finished updating images", "deployment", deployment.GetName())
}

// UpdateImageSpec updates the image of a with a new registry and tag
func UpdateImageSpec(image *caching.Image, registry *servingv1alpha1.Registry, log logr.Logger) {
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
