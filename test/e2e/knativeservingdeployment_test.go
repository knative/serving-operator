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
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	mf "github.com/jcrossley3/manifestival"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logstream"
	"knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
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
	if _, err := resources.CreateKnativeServing(clients.KnativeServing(), names); err != nil {
		t.Fatalf("KnativeService %q failed to create: %v", names.KnativeServing, err)
	}

	// Test if KnativeServing can reach the READY status
	t.Run("create", func(t *testing.T) {
		knativeServingVerify(t, clients, names)
	})

	t.Run("configure", func(t *testing.T) {
		knativeServingVerify(t, clients, names)
		knativeServingConfigure(t, clients, names)
	})

	// Delete the deployments one by one to see if they will be recreated.
	t.Run("restore", func(t *testing.T) {
		knativeServingVerify(t, clients, names)
		deploymentRecreation(t, clients, names)
	})

	// Delete the KnativeServing to see if all resources will be removed
	t.Run("delete", func(t *testing.T) {
		knativeServingVerify(t, clients, names)
		knativeServingDelete(t, clients, names)
	})
}

// knativeServingVerify verifies if the KnativeServing can reach the READY status.
func knativeServingVerify(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	if _, err := resources.WaitForKnativeServingState(clients.KnativeServing(), names.KnativeServing,
		resources.IsKnativeServingReady); err != nil {
		t.Fatalf("KnativeService %q failed to get to the READY status: %v", names.KnativeServing, err)
	}

}

// knativeServingConfigure verifies that KnativeServing config is set properly
func knativeServingConfigure(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	// We'll arbitrarily choose logging and defaults config
	loggingConfigKey := "logging"
	loggingConfigMapName := fmt.Sprintf("%s/config-%s", names.Namespace, loggingConfigKey)
	defaultsConfigKey := "defaults"
	defaultsConfigMapName := fmt.Sprintf("%s/config-%s", names.Namespace, defaultsConfigKey)
	// Get the existing KS without any spec
	ks, err := clients.KnativeServing().Get(names.KnativeServing, metav1.GetOptions{})
	// Add config to its spec
	ks.Spec = v1alpha1.KnativeServingSpec{
		Config: map[string]map[string]string{
			defaultsConfigKey: {
				"revision-timeout-seconds": "200",
			},
			loggingConfigKey: {
				"loglevel.controller": "debug",
				"loglevel.autoscaler": "debug",
			},
		},
	}
	// Update it
	if ks, err = clients.KnativeServing().Update(ks); err != nil {
		t.Fatalf("KnativeServing %q failed to update: %v", names.KnativeServing, err)
	}
	// Verify the relevant configmaps have been updated
	err = resources.WaitForConfigMap(defaultsConfigMapName, clients.KubeClient.Kube, func(m map[string]string) bool {
		return m["revision-timeout-seconds"] == "200"
	})
	if err != nil {
		t.Fatalf("The operator failed to update %s configmap", defaultsConfigMapName)
	}
	err = resources.WaitForConfigMap(loggingConfigMapName, clients.KubeClient.Kube, func(m map[string]string) bool {
		return m["loglevel.controller"] == "debug" && m["loglevel.autoscaler"] == "debug"
	})
	if err != nil {
		t.Fatalf("The operator failed to update %s configmap", loggingConfigMapName)
	}

	// Delete a single key/value pair
	delete(ks.Spec.Config[loggingConfigKey], "loglevel.autoscaler")
	// Update it
	if ks, err = clients.KnativeServing().Update(ks); err != nil {
		t.Fatalf("KnativeServing %q failed to update: %v", names.KnativeServing, err)
	}
	// Verify the relevant configmap has been updated
	err = resources.WaitForConfigMap(loggingConfigMapName, clients.KubeClient.Kube, func(m map[string]string) bool {
		_, autoscalerKeyExists := m["loglevel.autoscaler"]
		// deleted key/value pair should be removed from the target config map
		return m["loglevel.controller"] == "debug" && !autoscalerKeyExists
	})
	if err != nil {
		t.Fatal("The operator failed to update the configmap")
	}

	// Use an empty map as the value
	ks.Spec.Config[defaultsConfigKey] = map[string]string{}
	// Update it
	if ks, err = clients.KnativeServing().Update(ks); err != nil {
		t.Fatalf("KnativeServing %q failed to update: %v", names.KnativeServing, err)
	}
	// Verify the relevant configmap has been updated and does not contain any keys except "_example"
	err = resources.WaitForConfigMap(defaultsConfigMapName, clients.KubeClient.Kube, func(m map[string]string) bool {
		_, exampleExists := m["_example"]
		return len(m) == 1 && exampleExists
	})
	if err != nil {
		t.Fatal("The operator failed to update the configmap")
	}

	// Now remove the config from the spec and update
	ks.Spec = v1alpha1.KnativeServingSpec{}
	if ks, err = clients.KnativeServing().Update(ks); err != nil {
		t.Fatalf("KnativeServing %q failed to update: %v", names.KnativeServing, err)
	}
	// And verify the configmap entry is gone
	err = resources.WaitForConfigMap(loggingConfigMapName, clients.KubeClient.Kube, func(m map[string]string) bool {
		_, exists := m["loglevel.controller"]
		return !exists
	})
	if err != nil {
		t.Fatal("The operator failed to revert the configmap")
	}
}

// deploymentRecreation verify whether all the deployments for knative serving are able to recreate, when they are deleted.
func deploymentRecreation(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	dpList, err := clients.KubeClient.Kube.AppsV1().Deployments(names.Namespace).List(metav1.ListOptions{})
	if err != nil {
		t.Fatalf("Failed to get any deployment under the namespace %q: %v",
			test.ServingOperatorNamespace, err)
	}
	if len(dpList.Items) == 0 {
		t.Fatalf("No deployment under the namespace %q was found",
			test.ServingOperatorNamespace)
	}
	// Delete the first deployment and verify the operator recreates it
	deployment := dpList.Items[0]
	if err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Delete(deployment.Name,
		&metav1.DeleteOptions{}); err != nil {
		t.Fatalf("Failed to delete deployment %s/%s: %v", deployment.Namespace, deployment.Name, err)
	}

	waitErr := wait.PollImmediate(resources.Interval, resources.Timeout, func() (bool, error) {
		dep, err := clients.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Get(deployment.Name, metav1.GetOptions{})
		if err != nil {
			// If the deployment is not found, we continue to wait for the availability.
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return resources.IsDeploymentAvailable(dep)
	})

	if waitErr != nil {
		t.Fatalf("The deployment %s/%s failed to reach the desired state: %v", deployment.Namespace, deployment.Name, err)
	}

	if _, err := resources.WaitForKnativeServingState(clients.KnativeServing(), test.ServingOperatorName,
		resources.IsKnativeServingReady); err != nil {
		t.Fatalf("KnativeService %q failed to reach the desired state: %v", test.ServingOperatorName, err)
	}
	t.Logf("The deployment %s/%s reached the desired state.", deployment.Namespace, deployment.Name)
}

// knativeServingDelete deletes tha KnativeServing to see if all resources will be deleted
func knativeServingDelete(t *testing.T, clients *test.Clients, names test.ResourceNames) {
	if err := clients.KnativeServing().Delete(names.KnativeServing, &metav1.DeleteOptions{}); err != nil {
		t.Fatalf("KnativeServing %q failed to delete: %v", names.KnativeServing, err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mf.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "config/"), false, clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoKnativeServings(clients); err != nil {
		t.Fatal(err)
	}
	for _, u := range m.Resources {
		waitErr := wait.PollImmediate(resources.Interval, resources.Timeout, func() (bool, error) {
			gvrs, _ := meta.UnsafeGuessKindToResource(u.GroupVersionKind())
			if _, err := clients.Dynamic.Resource(gvrs).Get(u.GetName(), metav1.GetOptions{}); apierrs.IsNotFound(err) {
				return true, nil
			}
			return false, err
		})

		if waitErr != nil {
			t.Fatalf("The %s %s failed to be deleted: %v", u.GetKind(), u.GetName(), waitErr)
		}
		t.Logf("The %s %s has been deleted.", u.GetKind(), u.GetName())
	}
}

func verifyNoKnativeServings(clients *test.Clients) error {
	servings, err := clients.KnativeServingAll().List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(servings.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any KnativeServing exists")
	}
	return nil
}
