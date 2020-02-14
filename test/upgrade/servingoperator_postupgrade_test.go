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

package upgrade

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/test/logstream"

	"knative.dev/pkg/ptr"
	ptest "knative.dev/pkg/test"
	operatorTest "knative.dev/serving-operator/test"
	"knative.dev/serving-operator/test/client"
	"knative.dev/serving-operator/test/resources"
	v1 "knative.dev/serving/pkg/apis/serving/v1"
	serviceresourcenames "knative.dev/serving/pkg/reconciler/service/resources/names"
	"knative.dev/serving/test"
	"knative.dev/serving/test/e2e"
	v1test "knative.dev/serving/test/v1"
	v1a1test "knative.dev/serving/test/v1alpha1"
)

// TestKnativeServingPostUpgrade verifies the KnativeServing creation, deployment recreation, and KnativeServing deletion
// after the operator upgrades with the latest generated manifest of Knative Serving.
func TestKnativeServingPostUpgrade(t *testing.T) {
	cancel := logstream.Start(t)
	defer cancel()
	clients := client.Setup(t)

	names := operatorTest.ResourceNames{
		KnativeServing: operatorTest.ServingOperatorName,
		Namespace:      operatorTest.ServingOperatorNamespace,
	}

	operatorTest.CleanupOnInterrupt(func() { operatorTest.TearDown(clients, names) })
	defer operatorTest.TearDown(clients, names)

	// Create a KnativeServing custom resource, if it does not exist
	if _, err := resources.EnsureKnativeServingExists(clients.KnativeServing(), names); err != nil {
		t.Fatalf("KnativeService %q failed to create: %v", names.KnativeServing, err)
	}

	// Test if KnativeServing can reach the READY status after upgrade
	t.Run("create", func(t *testing.T) {
		resources.AssertKSOperatorCRReadyStatus(t, clients, names)
	})

	// Verify if resources match the latest requirement after upgrade
	t.Run("verify resources", func(t *testing.T) {
		resources.AssertKSOperatorCRReadyStatus(t, clients, names)
		// TODO: We only verify the deployment, but we need to add other resources as well, like ServiceAccount, ClusterRoleBinding, etc.
		expectedDeployments := []string{"networking-istio", "webhook", "controller", "activator", "autoscaler-hpa",
			"autoscaler"}
		ksVerifyDeployment(t, clients, names, expectedDeployments)
	})

	//t.Run("verify services", func(t *testing.T) {
	//	resources.AssertKSOperatorCRReadyStatus(t, clients, names)
	//	testRunLatestServicePostUpgrade(t)
	//	testRunLatestServicePostUpgradeFromScaleToZero(t)
	//	testBYORevisionPostUpgrade(t)
	//})

	// Delete the KnativeServing to see if all resources will be removed after upgrade
	t.Run("delete", func(t *testing.T) {
		resources.AssertKSOperatorCRReadyStatus(t, clients, names)
		resources.KSOperatorCRDelete(t, clients, names)
	})
}

// ksVerifyDeployment verify whether the deployments have the correct number and names.
func ksVerifyDeployment(t *testing.T, clients *operatorTest.Clients, names operatorTest.ResourceNames,
	expectedDeployments []string) {
	dpList, err := clients.KubeClient.Kube.AppsV1().Deployments(names.Namespace).List(metav1.ListOptions{})
	assertEqual(t, err, nil)
	assertEqual(t, len(dpList.Items), len(expectedDeployments))
	for _, deployment := range dpList.Items {
		assertEqual(t, stringInList(deployment.Name, expectedDeployments), true)
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

func testRunLatestServicePostUpgrade(t *testing.T) {
	updateService(serviceName, t)
}

func testRunLatestServicePostUpgradeFromScaleToZero(t *testing.T) {
	updateService(scaleToZeroServiceName, t)
}

// testBYORevisionPostUpgrade attempts to update the RouteSpec of a Service using BYO Revision name. This
// test is meant to catch new defaults that break the immutability of BYO Revision name.
func testBYORevisionPostUpgrade(t *testing.T) {
	clients := e2e.Setup(t)
	names := test.ResourceNames{
		Service: byoServiceName,
	}

	if _, err := v1test.UpdateServiceRouteSpec(t, clients, names, v1.RouteSpec{
		Traffic: []v1.TrafficTarget{{
			Tag:          "example-tag",
			RevisionName: byoRevName,
			Percent:      ptr.Int64(100),
		}},
	}); err != nil {
		t.Fatalf("Failed to update Service: %v", err)
	}
}

func updateService(serviceName string, t *testing.T) {
	t.Helper()
	clients := e2e.Setup(t)
	names := test.ResourceNames{
		Service: serviceName,
	}

	t.Logf("Getting service %q", names.Service)
	svc, err := clients.ServingAlphaClient.Services.Get(names.Service, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get Service: %v", err)
	}
	names.Route = serviceresourcenames.Route(svc)
	names.Config = serviceresourcenames.Configuration(svc)
	names.Revision = svc.Status.LatestCreatedRevisionName

	routeURL := svc.Status.URL.URL()

	t.Log("Check that we can hit the old service and get the old response.")
	assertServiceResourcesUpdated(t, clients, names, routeURL, test.PizzaPlanetText1)

	t.Log("Updating the Service to use a different image")
	newImage := ptest.ImagePath(test.PizzaPlanet2)
	if _, err := v1a1test.PatchServiceImage(t, clients, svc, newImage); err != nil {
		t.Fatalf("Patch update for Service %s with new image %s failed: %v", names.Service, newImage, err)
	}

	t.Log("Since the Service was updated a new Revision will be created and the Service will be updated")
	revisionName, err := v1a1test.WaitForServiceLatestRevision(clients, names)
	if err != nil {
		t.Fatalf("Service %s was not updated with the Revision for image %s: %v", names.Service, test.PizzaPlanet2, err)
	}
	names.Revision = revisionName

	t.Log("When the Service reports as Ready, everything should be ready.")
	if err := v1a1test.WaitForServiceState(clients.ServingAlphaClient, names.Service, v1a1test.IsServiceReady, "ServiceIsReady"); err != nil {
		t.Fatalf("The Service %s was not marked as Ready to serve traffic to Revision %s: %v", names.Service, names.Revision, err)
	}
	assertServiceResourcesUpdated(t, clients, names, routeURL, test.PizzaPlanetText2)
}
