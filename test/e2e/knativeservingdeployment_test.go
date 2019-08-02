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

	"knative.dev/pkg/test/helpers"
	"knative.dev/pkg/test/logstream"
	"knative.dev/serving-operator/test"
)

// TestKnativeServingDeployment verifies the KnativeServing creation, deployment recreation, and KnativeServing deletion.
func TestKnativeServingDeployment(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	clients := Setup(t)

	names := test.ResourceNames{
		KnativeServing: test.ServingOperatorName,
		Namespace:      helpers.AppendRandomString(test.ServingOperatorNamespace),
	}

	test.CleanupOnInterrupt(func() { test.TearDown(clients, names) })
	defer test.TearDown(clients, names)

	// Create the namespace for tests
	CreateNamespace(t, clients, names.Namespace)

	// Change the namespace for the clients
	clients = SetupWithNamespace(t, names.Namespace)

	// Create a KnativeServing to see if it can reach the READY status
	TestKnativeServingCreation(t, clients, names)

	// Delete the deployments one by one to see if they will be recreated.
	dpList := TestDeploymentRecreation(t, clients, names)

	// Delete the KnativeServing to see if all the deployments will be removed as well
	TestKnativeServingDeletion(t, clients, names, dpList)
}
