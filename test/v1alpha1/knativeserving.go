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

package v1alpha1

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

func WaitForKnativeServingState(client *test.KnativeServingAlphaClients, name string, inState func(s *v1alpha1.KnativeServing) (bool, error), desc string) (*v1alpha1.KnativeServing, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForKnativeServingState/%s/%s", name, desc))
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

func IsKnativeServingReady(s *v1alpha1.KnativeServing) (bool, error) {
	return s.Status.IsReady(), nil
}

func ListDeployment (clients *test.Clients, namespace string) (*v1.DeploymentList, error) {
	return clients.KubeClient.Kube.AppsV1().Deployments(namespace).List(metav1.ListOptions{})
}

func DeleteDeployment(client *test.Clients, deployment v1.Deployment) error {
	return client.KubeClient.Kube.AppsV1().Deployments(deployment.Namespace).Delete(deployment.Name, &va1.DeleteOptions{})
}

func WaitForDeploymentAvailable(clients *test.Clients, name, namespace string, inState func(s *v1.Deployment) (bool, error)) (*v1.Deployment, error) {
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
	return dep, nil
}

func IsDeploymentAvailable(d *v1.Deployment) (bool, error) {
	for _, dc := range d.Status.Conditions {
		return dc.Type == "Available" && dc.Status == "True", nil
	}
	return false, nil
}
