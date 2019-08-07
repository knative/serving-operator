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

package e2e

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// Mysteriously required to support GCP auth (required by k8s libs).
	// Apparently just importing it is enough. @_@ side effects @_@.
	// https://github.com/kubernetes/client-go/issues/242
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	pkgTest "knative.dev/pkg/test"
	"knative.dev/serving-operator/test"
	"knative.dev/serving-operator/test/resources"
)

// Setup creates the client objects needed in the e2e tests.
func Setup(t *testing.T) *test.Clients {
	return SetupWithNamespace(t, "default")
}

// SetupWithNamespace creates the client objects needed in the e2e tests under the specified namespace.
func SetupWithNamespace(t *testing.T, namespace string) *test.Clients {
	clients, err := test.NewClients(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster,
		namespace)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}
	return clients
}

// CreateNamespace creates a namespace to run the integration tests.
func CreateNamespace(t *testing.T, clients *test.Clients, namespace string) {
	_, err := clients.KubeClient.Kube.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil && apierrs.IsNotFound(err) {
		// Create the namespace if not available
		if _, err = clients.KubeClient.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta:
			metav1.ObjectMeta{Name: namespace}}); err != nil {
			t.Fatalf("Failed to create the namespace %s: %v", namespace, err)
		}
	}
}

// KnativeServingVerify verifies if the KnativeServing can reach the READY status.
func knativeServingVerify(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	if _, err := resources.WaitKnativeServingReady(t, clients.KnativeServingAlphaClient, names.KnativeServing,
		resources.IsKnativeServingReady); err != nil {
		t.Fatalf("KnativeService %q failed to get to the READY status: %v", names.KnativeServing, err)
	}

}

// DeploymentRecreation verify whether all the deployments for knative serving are able to recreate, when they are deleted.
func DeploymentRecreation(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	dpListSave, err := clients.KubeClient.Kube.AppsV1().Deployments(names.Namespace).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to get any deployment under the namespace %q: %v",
			test.ServingOperatorNamespace, err)
	}
	if len(dpListSave.Items) == 0 {
		t.Fatalf("No deployment under the namespace %q was found",
			test.ServingOperatorNamespace)
	}
	// Delete the deployments one by one to see if they will be recreated.
	for _, deployment := range dpListSave.Items {
		if err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Delete(deployment.Name,
			&metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Failed to delete deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
		}
		if _, err = resources.WaitForDeployment(clients, deployment.Name, deployment.Namespace,
			resources.IsDeploymentAvailable, "DeploymentIsAvailable"); err != nil {
			t.Fatalf("The deployment %s/%s failed to reach the desired state: %v",
				deployment.Namespace, deployment.Name, err)
		}
		if _, err := resources.WaitForKnativeServingState(clients.KnativeServingAlphaClient, test.ServingOperatorName,
			resources.IsKnativeServingReady); err != nil {
			t.Fatalf("KnativeService %q failed to reach the desired state: %v", test.ServingOperatorName, err)
		}
		t.Logf("The deployment %s/%s reached the desired state.", deployment.Namespace, deployment.Name)
	}
}

// KnativeServingDeletion deletes tha KnativeServing to see if all the deployments will be removed.
func KnativeServingDeletion(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	if err := clients.KnativeServingAlphaClient.Delete(names.KnativeServing, &metav1.DeleteOptions{}); err != nil {
		t.Fatalf("KnativeService %q failed to delete: %v", names.KnativeServing, err)
	}
	if _, err := resources.WaitForKnativeServingState(clients.KnativeServingAlphaClient, names.KnativeServing,
		resources.IsKnativeServingDeleted); err != nil {
		t.Fatalf("KnativeService %q failed to be deleted: %v", names.KnativeServing, err)
	}

	dpListSave, err := clients.KubeClient.Kube.AppsV1().Deployments(names.Namespace).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Error getting any deployment under the namespace %q: %v", names.Namespace, err)
	}

	for _, deployment := range dpListSave.Items {
		if _, err := resources.WaitForDeployment(clients, deployment.Name, deployment.Namespace,
			resources.IsDeploymentDeleted, "DeploymentIsDeleted"); err != nil {
			t.Fatalf("The deployment %s/%s failed to be deleted: %v",
				deployment.Namespace, deployment.Name, err)
		}
		t.Logf("The deployment %s/%s has been deleted.", deployment.Namespace, deployment.Name)
	}
}
