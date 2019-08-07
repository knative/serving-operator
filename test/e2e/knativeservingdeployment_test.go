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

	"knative.dev/pkg/test/logstream"
	"knative.dev/serving-operator/test"
	"knative.dev/serving-operator/test/resources"
)

// TestKnativeServingDeployment verifies the KnativeServing creation, deployment recreation, and KnativeServing deletion.
func TestKnativeServingDeployment(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	clients := Setup(t)

	names := test.ResourceNames{
		KnativeServing: test.ServingOperatorName,
		Namespace:      test.ServingOperatorNamespace,
	}

	test.CleanupOnInterrupt(func() { test.TearDown(clients, names) })
	defer test.TearDown(clients, names)

	// Create a KnativeServing
	if _, err := resources.CreateKnativeServing(clients.KnativeServingAlphaClient, names); err != nil {
		t.Fatalf("KnativeService %q failed to create: %v", names.KnativeServing, err)
	}

	// Test if KnativeServing can reach the READY status
	t.Run("create", func(t *testing.T) {
		knativeServingVerify(t, clients, names)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore", func(t *testing.T) {
		knativeServingVerify(t, clients, names)
		deploymentRecreation(t, clients, names)
	})

	// Delete the KnativeServing to see if all the deployments will be removed as well
	t.Run("delete", func(t *testing.T) {
		knativeServingVerify(t, clients, names)
		knativeServingDeletion(t, clients, names)
	})
}
