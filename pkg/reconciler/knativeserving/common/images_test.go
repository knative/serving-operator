package common

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	caching "knative.dev/caching/pkg/apis/caching/v1alpha1"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
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

func TestDeploymentTransform(t *testing.T) {
	for _, tt := range updateDeploymentImageTests {
		t.Run(tt.name, func(t *testing.T) {
			runDeploymentTransformTest(t, &tt)
		})
	}
}
func runDeploymentTransformTest(t *testing.T, tt *updateDeploymentImageTest) {
	log := logf.Log.WithName(tt.name)
	logf.SetLogger(logf.ZapLogger(true))
	testScheme := runtime.NewScheme()
	unstructuredDeployment := makeUnstructuredDeployment(t, tt, testScheme)
	instance := &servingv1alpha1.KnativeServing{
		Spec: servingv1alpha1.KnativeServingSpec{
			Registry: tt.registry,
		},
	}
	deploymentTransform := DeploymentTransform(testScheme, instance, log)
	deploymentTransform(&unstructuredDeployment)
	validateUnstructedDeploymentChanged(t, tt, &unstructuredDeployment)
}

func validateUnstructedDeploymentChanged(t *testing.T, tt *updateDeploymentImageTest, u *unstructured.Unstructured) {
	var deployment = &appsv1.Deployment{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, deployment)
	assertEqual(t, err, nil)
	for i, expected := range tt.expected {
		assertEqual(t, deployment.Spec.Template.Spec.Containers[i].Image, expected)
	}
}

func makeUnstructuredDeployment(t *testing.T, tt *updateDeploymentImageTest, scheme *runtime.Scheme) unstructured.Unstructured {
	deployment := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
		},
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
	unstructuredDeployment, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&deployment)
	if err != nil {
		t.Fatalf("Could not create unstructured deployment object: %v, err: %v", unstructuredDeployment, err)
	}
	return unstructured.Unstructured{
		Object: unstructuredDeployment,
	}
}

type updateImageSpecTest struct {
	name     string
	in       string
	registry servingv1alpha1.Registry
	expected string
}

var updateImageSpecTests = []updateImageSpecTest{
	{
		name: "UsesNameFromDefault",
		in:   "gcr.io/knative-releases/github.com/knative/serving/cmd/queue@sha256:1e40c99ff5977daa2d69873fff604c6d09651af1f9ff15aadf8849b3ee77ab45",
		registry: servingv1alpha1.Registry{
			Default: "new-registry.io/test/path/${NAME}:new-tag",
		},
		expected: "new-registry.io/test/path/UsesNameFromDefault:new-tag",
	},
}

func TestImageTransform(t *testing.T) {
	for _, tt := range updateImageSpecTests {
		t.Run(tt.name, func(t *testing.T) {
			runImageTransformTest(t, &tt)
		})
	}
}
func runImageTransformTest(t *testing.T, tt *updateImageSpecTest) {
	log := logf.Log.WithName(tt.name)
	logf.SetLogger(logf.ZapLogger(true))

	testScheme := runtime.NewScheme()
	unstructuredImage := makeUnstructuredImage(t, tt, testScheme)
	instance := &servingv1alpha1.KnativeServing{
		Spec: servingv1alpha1.KnativeServingSpec{
			Registry: tt.registry,
		},
	}
	imageTransform := ImageTransform(testScheme, instance, log)
	imageTransform(&unstructuredImage)
	validateUnstructedImageChanged(t, tt, &unstructuredImage)
}

func validateUnstructedImageChanged(t *testing.T, tt *updateImageSpecTest, u *unstructured.Unstructured) {
	var image = &caching.Image{}
	err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, image)
	assertEqual(t, err, nil)
	assertEqual(t, image.Spec.Image, tt.expected)
}

func makeUnstructuredImage(t *testing.T, tt *updateImageSpecTest, scheme *runtime.Scheme) unstructured.Unstructured {
	image := caching.Image{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "caching.internal.knative.dev/v1alpha1",
			Kind:       "Image",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: tt.name,
		},
		Spec: caching.ImageSpec{
			Image: tt.in,
		},
	}
	unstructuredDeployment, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&image)
	if err != nil {
		t.Fatalf("Could not create unstructured deployment object: %v, err: %v", unstructuredDeployment, err)
	}
	return unstructured.Unstructured{
		Object: unstructuredDeployment,
	}
}

func assertEqual(t *testing.T, actual, expected interface{}) {
	if actual == expected {
		return
	}
	t.Fatalf("expected does not equal actual. \nExpected: %v\nActual: %v", expected, actual)
}
