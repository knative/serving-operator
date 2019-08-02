// +build e2e

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/logstream"
	"knative.dev/serving-operator/test"
	"knative.dev/serving-operator/test/resources"
)

// TestKnativeServingDeploymentRecreationReady verifies whether the deployment is recreated, if it is deleted.
func TestKnativeServingDeploymentRecreationReady(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	clients := Setup(t)

	dpList, err := clients.KubeClient.Kube.AppsV1().Deployments(test.ServingOperatorNamespace).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to get any deployment under the namespace %q: %v",
			test.ServingOperatorNamespace, err)
	}
	if len(dpList.Items) == 0 {
		t.Fatalf("No deployment under the namespace %q was found",
			test.ServingOperatorNamespace)
	}
	// Delete the deployments one by one to see if they will be recreated.
	for _, deployment := range dpList.Items {
		if err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Delete(deployment.Name,
			&metav1.DeleteOptions{}); err != nil {
			t.Fatalf("Failed to delete deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
		}
		if _, err = resources.WaitForDeploymentAvailable(clients, deployment.Name, deployment.Namespace,
			resources.IsDeploymentAvailable); err != nil {
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
