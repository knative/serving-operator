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
	"time"

	"github.com/knative/serving-operator/pkg/apis"
	servingv1alpha1 "github.com/knative/serving-operator/pkg/apis/serving/v1alpha1"
	framework "github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/operator-framework/operator-sdk/pkg/test/e2eutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	retryInterval        = time.Second * 5
	timeout              = time.Second * 60
	cleanupRetryInterval = time.Second * 1
	cleanupTimeout       = time.Second * 5
)

func TestKnativeServing(t *testing.T) {
	knativeServingList := &servingv1alpha1.KnativeServingList{
		TypeMeta: metav1.TypeMeta{
			Kind:       "KnativeServing",
			APIVersion: "serving.knative.dev/v1alpha1",
		},
	}

	err := framework.AddToFrameworkScheme(apis.AddToScheme, knativeServingList)
	if err != nil {
		t.Fatalf("failed to add custom resource scheme to framework: %v", err)
	}

	// run subtests in subgroups, maybe? we could re-organize this
	// however it makes sense.
	t.Run("knative-serving-group", func(t *testing.T) {
		t.Run("Cluster", KnativeServingCluster)
	})
}

func KnativeServingCluster(t *testing.T) {
	ctx := framework.NewTestCtx(t)
	defer ctx.Cleanup()
	err := ctx.InitializeClusterResources(&framework.CleanupOptions{TestContext: ctx, Timeout: cleanupTimeout, RetryInterval: cleanupRetryInterval})
	if err != nil {
		t.Fatalf("failed to initialize cluster resources: %v", err)
	}
	t.Log("Initialized cluster resources")
	namespace, err := ctx.GetNamespace()
	if err != nil {
		t.Fatal(err)
	}
	// get global framework variables
	f := framework.Global
	err = e2eutil.WaitForDeployment(t, f.KubeClient, namespace, "knative-serving-operator", 1, retryInterval, timeout)
	if err != nil {
		t.Fatal(err)
	}
	// Operator should auto-install knative-serving, so wait on deployments
	deployments := []string{"controller", "activator", "autoscaler", "webhook"}
	for _, name := range deployments {
		err = e2eutil.WaitForDeployment(t, f.KubeClient, "knative-serving", name, 1, retryInterval, timeout)
		if err != nil {
			t.Fatal(err)
		}
	}
}
