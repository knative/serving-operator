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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/logstream"
	"knative.dev/serving-operator/test"
	"knative.dev/serving-operator/test/resources"
	"testing"
)

// TestKnativeServingDeploymentRecreationReady verifies whether the deployment is recreated, if it is deleted.
func TestKnativeServingDeploymentRecreationReady(t *testing.T) {
	t.Parallel()
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
	} else {
		// Delete a random deployment to see if they will be recreated.
		//deployment := dpList.Items[rand.Intn(len(dpList.Items))]
		for i, deployment := range dpList.Items {
			err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Delete(deployment.Name,
				&metav1.DeleteOptions{})
			if err != nil {
				t.Fatalf("Failed to delete deployment %s/%s: %v",
					deployment.Namespace, deployment.Name, err)
			}
			_, err = resources.WaitForDeploymentAvailable(clients, deployment.Name, deployment.Namespace,
				resources.IsDeploymentAvailable)
			if err != nil {
				t.Fatalf("The deployment %s/%s failed to reach the desired state: %v",
					deployment.Namespace, deployment.Name, err)
			}
			t.Logf("The deployment %s/%s reached the desired state.", deployment.Namespace, deployment.Name)
			if i < len(dpList.Items) - 1 {
				// When the deployment revives, wait additional 20 seconds for it to be stabilized, before
				// delete another deployment.
				time.Sleep(20 * time.Second)
			}
		}
	}
}
