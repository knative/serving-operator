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

	"knative.dev/pkg/test/logstream"
	"knative.dev/serving-operator/test"
	"knative.dev/serving-operator/test/v1alpha1"
	"testing"
)

func TestKnativeServingDeploymentRecreationReady(t *testing.T) {
	t.Parallel()
	cancel := logstream.Start(t)
	defer cancel()
	clients := Setup(t)

	if dpList, err := v1alpha1.ListDeployment(clients, test.ServingOperatorNamespace); err != nil {
		t.Fatalf("Failed to get any deployment under the namespace %v was found: %v",
			test.ServingOperatorNamespace, err)
	} else if len(dpList.Items) == 0 {
		t.Fatalf("No deployment under the namespace %v was found: %v",
			test.ServingOperatorNamespace, err)
	} else {
		// Delete a random deployment to see if they will be recreated.
		//deployment := dpList.Items[rand.Intn(len(dpList.Items))]
		for i, deployment := range dpList.Items {
			error := v1alpha1.DeleteDeployment(clients, deployment)
			if error != nil {
				t.Fatalf("Failed to delete the deployment %v under the namespace %v was found: %v",
					deployment.Name, deployment.Namespace, error)
			}
			_, err := v1alpha1.WaitForDeploymentAvailable(clients, deployment.Name, deployment.Namespace,
				v1alpha1.IsDeploymentAvailable)
			if err != nil {
				t.Fatalf("The deployment %v under the namespace %v failed to reach the desired state: %v",
					deployment.Name, deployment.Namespace, err)
			} else {
				t.Logf("The deployment %v under the namespace %v reached the desired state.",
					deployment.Name, deployment.Namespace)
			}
			if i < len(dpList.Items) - 1 {
				// When the deployment revives, wait additional 20 seconds for it to be stabilized, before
				// delete another deployment.
				time.Sleep(20 * time.Second)
			}
		}
	}
}
