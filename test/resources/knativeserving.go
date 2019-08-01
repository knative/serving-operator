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

// knativeserving.go provides methods to perform actions on the KnativeServing resource.

package resources

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/apps/v1"
	va1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"github.com/pkg/errors"
	"knative.dev/pkg/test/logging"
	"knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	"knative.dev/serving-operator/test"
)

const (
	interval = 10 * time.Second
	timeout  = 2 * time.Minute
)

// WaitForKnativeServingState polls the status of the KnativeServing called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForKnativeServingState(client *test.KnativeServingAlphaClients, name string, inState func(s *v1alpha1.KnativeServing) (bool, error)) (*v1alpha1.KnativeServing, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForKnativeServingState/%s/%s", name, "KnativeServingIsReady"))
	defer span.End()

	var lastState *v1alpha1.KnativeServing
	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		lastState, _ = client.KnativeServings.Get(name, metav1.GetOptions{})
		return inState(lastState)
	})

	if waitErr != nil {
		return lastState, errors.Wrapf(waitErr, "knativeserving %q is not in desired state, got: %+v", name, lastState)
	}
	return lastState, nil
}

// IsKnativeServingReady will check the status conditions of the KnativeServing and return true if the KnativeServing is ready.
func IsKnativeServingReady(s *v1alpha1.KnativeServing) (bool, error) {
	return s.Status.IsReady(), nil
}

// WaitForDeploymentAvailable polls the status of the deployment called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout. ownerKnativeServing specifies the
// name of the owner KnativeServing, since we verify the state of KnativeServing as well.
func WaitForDeploymentAvailable(clients *test.Clients, name, namespace string, inState func(s *v1.Deployment) (bool, error),
	ownerKnativeServing string) (*v1.Deployment, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForKnativeServingState/%s/%s", name,
		"DeploymentIsAvailable"))
	defer span.End()
	var dep *v1.Deployment

	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		dep, _ = clients.KubeClient.Kube.AppsV1().Deployments(namespace).Get(name, va1.GetOptions{})
		return inState(dep)
	})

	if waitErr != nil {
		return dep, errors.Wrapf(waitErr, "Deployment %q is not in desired state, got: %+v", name, dep)
	}

	_, err := WaitForKnativeServingState(clients.KnativeServingAlphaClient, ownerKnativeServing,
		IsKnativeServingReady)
	if err != nil {
		return dep, err
	}

	return dep, nil
}

// IsDeploymentAvailable will check the status conditions of the deployment and return true if the deployment is available.
func IsDeploymentAvailable(d *v1.Deployment) (bool, error) {
	for _, dc := range d.Status.Conditions {
		return dc.Type == "Available" && dc.Status == "True", nil
	}
	return false, nil
}
