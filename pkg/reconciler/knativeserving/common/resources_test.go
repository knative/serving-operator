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

package common

import (
	"reflect"
	"testing"

	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/kubernetes/scheme"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	"sigs.k8s.io/yaml"
)

var testdata = []byte(`
- apiVersion: operator.knative.dev/v1alpha1
  kind: KnativeServing
  metadata:
    name: single-container
  spec:
    resources:
      activator:
        requests:
          cpu: 330m
          memory: 69Mi
        limits:
          cpu: 9990m
          memory: 690Mi
- apiVersion: operator.knative.dev/v1alpha1
  kind: KnativeServing
  metadata:
    name: multi-container
  spec:
    resources:
      webhook:
        requests:
          cpu: 22m
          memory: 22Mi
        limits:
          cpu: 220m
          memory: 220Mi
      another:
        requests:
          cpu: 33m
          memory: 42Mi
        limits:
          cpu: 330m
          memory: 420Mi
- apiVersion: operator.knative.dev/v1alpha1
  kind: KnativeServing
  metadata:
    name: multi-deployment
  spec:
    resources:
      autoscaler:
        requests:
          cpu: 33m
          memory: 42Mi
        limits:
          cpu: 330m
          memory: 420Mi
      controller:
        requests:
          cpu: 999m
          memory: 999Mi
        limits:
          cpu: 9990m
          memory: 9990Mi
`)

func TestResourceRequirementsTransform(t *testing.T) {
	tests := []servingv1alpha1.KnativeServing{}
	err := yaml.Unmarshal(testdata, &tests)
	if err != nil {
		t.Error(err)
		return
	}
	for _, ks := range tests {
		t.Run(ks.Name, func(t *testing.T) {
			runResourceRequirementsTransformTest(t, &ks)
		})
	}
}

func runResourceRequirementsTransformTest(t *testing.T, ks *servingv1alpha1.KnativeServing) {
	manifest, err := mf.NewManifest("testdata/manifest.yaml")
	if err != nil {
		t.Error(err)
	}
	actual, err := manifest.Transform(ResourceRequirementsTransform(ks, log))
	if err != nil {
		t.Error(err)
	}
	for _, u := range actual.Filter(mf.ByKind("Deployment")).Resources() {
		deployment := &appsv1.Deployment{}
		if err := scheme.Scheme.Convert(&u, deployment, nil); err != nil {
			t.Error(err)
		}
		containers := deployment.Spec.Template.Spec.Containers
		for i := range containers {
			expected := ks.Spec.Resources[containers[i].Name]
			if !reflect.DeepEqual(containers[i].Resources, expected) {
				t.Errorf("Expected %v, Got %v", expected, containers[i].Resources)
			}
		}
	}
}
