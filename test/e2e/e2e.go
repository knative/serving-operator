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
	"k8s.io/apimachinery/pkg/util/wait"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
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
	return SetupWithNamespace(t, test.ServingOperatorNamespace)
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

// knativeServingVerify verifies if the KnativeServing can reach the READY status.
func knativeServingVerify(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	if _, err := resources.WaitKnativeServingReady(t, clients.KnativeServingAlphaClient, names.KnativeServing,
		resources.IsKnativeServingReady); err != nil {
		t.Fatalf("KnativeService %q failed to get to the READY status: %v", names.KnativeServing, err)
	}

}

// deploymentRecreation verify whether all the deployments for knative serving are able to recreate, when they are deleted.
func deploymentRecreation(t *testing.T, clients *test.Clients, names test.ResourceNames) {
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

		waitErr := wait.PollImmediate(resources.Interval, resources.Timeout, func() (bool, error) {
			dep, err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Get(deployment.Name, metav1.GetOptions{})
			if err != nil && apierrs.IsNotFound(err) {
				// If the deployment is not found, we continue to wait for the availability.
				return false, nil
			}
			return resources.IsDeploymentAvailable(dep)
		})

		if waitErr != nil {
			t.Fatalf("The deployment %s/%s failed to reach the desired state: %v", deployment.Namespace, deployment.Name, err)
		}

		if _, err := resources.WaitForKnativeServingState(clients.KnativeServingAlphaClient, test.ServingOperatorName,
			resources.IsKnativeServingReady); err != nil {
			t.Fatalf("KnativeService %q failed to reach the desired state: %v", test.ServingOperatorName, err)
		}
		t.Logf("The deployment %s/%s reached the desired state.", deployment.Namespace, deployment.Name)
	}
}

// knativeServingDeletion deletes tha KnativeServing to see if all the deployments will be removed.
func knativeServingDeletion(t *testing.T, clients *test.Clients, names test.ResourceNames) {
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
		waitErr := wait.PollImmediate(resources.Interval, resources.Timeout, func() (bool, error) {
			if _, err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Get(deployment.Name,
				metav1.GetOptions{}); err != nil && apierrs.IsNotFound(err) {
				// If the deployment is not found, we determine it is deleted.
				return true, nil
			}
			return false, nil
		})

		if waitErr != nil {
			t.Fatalf("The deployment %s/%s failed to be deleted: %v", deployment.Namespace, deployment.Name, err)
		}
		t.Logf("The deployment %s/%s has been deleted.", deployment.Namespace, deployment.Name)
	}
}
