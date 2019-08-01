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

// TestKnativeServingReady verifies whether KnativeServing is ready after the operator is installed.
func TestKnativeServingReady(t *testing.T) {
	t.Parallel()
	cancel := logstream.Start(t)
	defer cancel()
	clients := Setup(t)

	// Get the KnativeServing under knative-serving for tests
	// Since all the resources are limited under the namespace knative-serving for this operator,
	// we have specify both of the name and the namespace to knative-serving for KnativeServing.
	names := test.ResourceNames{
		KnativeServing: test.ServingOperatorName,
	}

	if _, err := resources.WaitForKnativeServingState(clients.KnativeServingAlphaClient, names.KnativeServing,
		resources.IsKnativeServingReady); err != nil {
		t.Fatalf("KnativeService %q failed to reach the desired state: %v", names.KnativeServing, err)
	}
}
