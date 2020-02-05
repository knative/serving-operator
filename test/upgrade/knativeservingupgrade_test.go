// +build postupgrade

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

package e2e

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/logstream"
	"knative.dev/serving-operator/test"
	"knative.dev/serving-operator/test/resources"
	"knative.dev/serving-operator/test/common"
)

// TestKnativeServingUpgrade verifies the KnativeServing creation, deployment recreation, and KnativeServing deletion.
func TestKnativeServingUpgrade(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	clients := common.Setup(t)

	names := test.ResourceNames{
		KnativeServing: test.ServingOperatorName,
		Namespace:      test.ServingOperatorNamespace,
	}

	test.CleanupOnInterrupt(func() { test.TearDown(clients, names) })
	defer test.TearDown(clients, names)

	// Create a KnativeServing
	if _, err := resources.CreateKnativeServing(clients.KnativeServing(), names); err != nil {
		t.Fatalf("KnativeService %q failed to create: %v", names.KnativeServing, err)
	}

	// Test if KnativeServing can reach the READY status
	t.Run("create", func(t *testing.T) {
		common.KnativeServingVerify(t, clients, names)
		knativeServingVerifyDeployment(t, clients, names)
	})

	t.Run("configure", func(t *testing.T) {
		common.KnativeServingVerify(t, clients, names)
		common.KnativeServingConfigure(t, clients, names)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore", func(t *testing.T) {
		common.KnativeServingVerify(t, clients, names)
		common.DeploymentRecreation(t, clients, names)
	})

	// Delete the KnativeServing to see if all resources will be removed
	t.Run("delete", func(t *testing.T) {
		common.KnativeServingVerify(t, clients, names)
		common.KnativeServingDelete(t, clients, names)
	})
}

// knativeServingVerifyDeployment verify whether the deployments have the correct number and names.
func knativeServingVerifyDeployment(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	// Knative Serving has 6 deployments.
	expectedNumDeployments := 6
	deploys := []string{"networking-istio", "webhook", "controller", "activator", "autoscaler-hpa", "autoscaler"}
	dpList, err := clients.KubeClient.Kube.AppsV1().Deployments(names.Namespace).List(metav1.ListOptions{})
	assertEqual(t, err, nil)
	assertEqual(t, expectedNumDeployments, len(dpList.Items))
	for _, deployment := range dpList.Items {
		assertEqual(t, stringInList(deployment.Name, deploys), true)
	}
}

func assertEqual(t *testing.T, actual, expected interface{}) {
	if actual == expected {
		return
	}
	t.Fatalf("expected does not equal actual. \nExpected: %v\nActual: %v", expected, actual)
}

func stringInList(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}
