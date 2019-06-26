package common

import (
	"testing"

	servingv1alpha1 "github.com/knative/serving-operator/pkg/apis/serving/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	caching "knative.dev/caching/pkg/apis/caching/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	appsv1 "k8s.io/api/apps/v1"
)

type updateDeploymentImageTest struct {
	name       string
	containers []corev1.Container
	registry   servingv1alpha1.Registry
	expected   []string
}

var updateDeploymentImageTests = []updateDeploymentImageTest{
	{
		name: "UsesNameFromDefault",
		containers: []corev1.Container{{
			Name:  "queue",
			Image: "gcr.io/knative-releases/github.com/knative/serving/cmd/queue@sha256:1e40c99ff5977daa2d69873fff604c6d09651af1f9ff15aadf8849b3ee77ab45"},
		},
		registry: servingv1alpha1.Registry{
			Default: "new-registry.io/test/path/${NAME}:new-tag",
		},
		expected: []string{"new-registry.io/test/path/queue:new-tag"},
	},
	{
		name: "UsesContainerNamePerContainer",
		containers: []corev1.Container{
			{
				Name:  "container1",
				Image: "gcr.io/cmd/queue:test",
			},
			{
				Name:  "container2",
				Image: "gcr.io/cmd/queue:test",
			},
		},
		registry: servingv1alpha1.Registry{
			Override: map[string]string{
				"container1": "new-registry.io/test/path/new-container-1:new-tag",
				"container2": "new-registry.io/test/path/new-container-2:new-tag",
			},
		},
		expected: []string{
			"new-registry.io/test/path/new-container-1:new-tag",
			"new-registry.io/test/path/new-container-2:new-tag",
		},
	},
	{
		name: "UsesOverrideFromDefault",
		containers: []corev1.Container{{
			Name:  "queue",
			Image: "gcr.io/knative-releases/github.com/knative/serving/cmd/queue@sha256:1e40c99ff5977daa2d69873fff604c6d09651af1f9ff15aadf8849b3ee77ab45"},
		},
		registry: servingv1alpha1.Registry{
			Default: "new-registry.io/test/path/${NAME}:new-tag",
			Override: map[string]string{
				"queue": "new-registry.io/test/path/new-value:new-override-tag",
			},
		},
		expected: []string{"new-registry.io/test/path/new-value:new-override-tag"},
	},
	{
		name: "NoChangeOverrideWithDifferentName",
		containers: []corev1.Container{{
			Name:  "image",
			Image: "docker.io/name/image:tag2"},
		},
		registry: servingv1alpha1.Registry{
			Override: map[string]string{
				"Unused": "new-registry.io/test/path",
			},
		},
		expected: []string{"docker.io/name/image:tag2"},
	},
	{
		name: "NoChange",
		containers: []corev1.Container{{
			Name:  "queue",
			Image: "gcr.io/knative-releases/github.com/knative/serving/cmd/queue@sha256:1e40c99ff5977daa2d69873fff604c6d09651af1f9ff15aadf8849b3ee77ab45"},
		},
		registry: servingv1alpha1.Registry{},
		expected: []string{"gcr.io/knative-releases/github.com/knative/serving/cmd/queue@sha256:1e40c99ff5977daa2d69873fff604c6d09651af1f9ff15aadf8849b3ee77ab45"},
	},
}

func TestUpdateDeploymentImage(t *testing.T) {
	for _, tt := range updateDeploymentImageTests {
		t.Run(tt.name, func(t *testing.T) {
			runUpdateDeploymentImageTest(t, tt)
		})
	}
}
func runUpdateDeploymentImageTest(t *testing.T, tt updateDeploymentImageTest) {
	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: tt.name,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: tt.containers,
				},
			},
		},
	}
	log := logf.Log.WithName(tt.name)
	logf.SetLogger(logf.ZapLogger(true))

	UpdateImage(&deployment, &tt.registry, log)

	for i, expected := range tt.expected {
		assertEqual(t, deployment.Spec.Template.Spec.Containers[i].Image, expected)
	}
}

func assertEqual(t *testing.T, actual, expected string) {
	if actual == expected {
		return
	}
	t.Fatalf("expected does not equal actual. \nExpected: %v\nActual: %v", expected, actual)
}
