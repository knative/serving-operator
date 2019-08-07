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
	"testing"
	"time"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"github.com/pkg/errors"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	va1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"
	"knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	"knative.dev/serving-operator/test"
)

const (
	interval = 10 * time.Second
	timeout  = 5 * time.Minute
)

// WaitKnativeServingReady polls the status of the KnativeServing called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitKnativeServingReady(t *testing.T, clients *test.KnativeServingAlphaClients, name string,
	inState func(s *v1alpha1.KnativeServing, err error) (bool, error)) (*v1alpha1.KnativeServing, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForKnativeServingState/%s/%s", name, "KnativeServingIsReady"))
	defer span.End()

	var lastState *v1alpha1.KnativeServing
	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		lastState, err := clients.KnativeServings.Get(name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, errors.Wrapf(waitErr, "knativeserving %s is not in desired state, got: %+v", name, lastState)
	}
	return lastState, nil
}

// CreateKnativeServing creates a KnativeServing with the name names.KnativeServing under the namespace names.Namespace.
func CreateKnativeServing(clients *test.KnativeServingAlphaClients, names test.ResourceNames) (*v1alpha1.KnativeServing, error) {
	ks := &v1alpha1.KnativeServing{
		ObjectMeta: metav1.ObjectMeta{
			Name: names.KnativeServing,
			Namespace: names.Namespace,
		},
	}
	svc, err := clients.KnativeServings.Create(ks)
	return svc, err
}

// WaitForKnativeServingState polls the status of the KnativeServing called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForKnativeServingState(client *test.KnativeServingAlphaClients, name string, inState func(s *v1alpha1.KnativeServing,
	err error) (bool, error)) (*v1alpha1.KnativeServing, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForKnativeServingState/%s/%s", name, "KnativeServingIsReady"))
	defer span.End()

	var lastState *v1alpha1.KnativeServing
	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		lastState, err := client.KnativeServings.Get(name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, errors.Wrapf(waitErr, "knativeserving %q is not in desired state, got: %+v", name, lastState)
	}
	return lastState, nil
}

// IsKnativeServingReady will check the status conditions of the KnativeServing and return true if the KnativeServing is ready.
func IsKnativeServingReady(s *v1alpha1.KnativeServing, err error) (bool, error) {
	return s.Status.IsReady(), nil
}

// IsKnativeServingDeleted will check the status conditions of the KnativeServing and return true if the KnativeServing is deleted.
func IsKnativeServingDeleted(s *v1alpha1.KnativeServing, err error) (bool, error) {
	return apierrs.IsNotFound(err), nil
}

// WaitForDeployment polls the status of the deployment called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForDeployment(clients *test.Clients, name, namespace string, inState func(s *v1.Deployment, err error) (bool, error),
	desc string) (*v1.Deployment, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForKnativeServingState/%s/%s", name,
		desc))
	defer span.End()
	var dep *v1.Deployment

	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		dep, err := clients.KubeClient.Kube.AppsV1().Deployments(namespace).Get(name, va1.GetOptions{})
		return inState(dep, err)
	})

	if waitErr != nil {
		return dep, errors.Wrapf(waitErr, "Deployment %q is not in desired status for the condition type Available,"+
			"got: %+q; want %+q", name, getDeploymentStatus(dep), "True")
	}

	return dep, nil
}

// IsDeploymentAvailable will check the status conditions of the deployment and return true if the deployment is available.
func IsDeploymentAvailable(d *v1.Deployment, err error) (bool, error) {
	return getDeploymentStatus(d) == "True", nil
}

// IsDeploymentAvailable will check the status conditions of the deployment and return true if the deployment is available.
func IsDeploymentDeleted(d *v1.Deployment, err error) (bool, error) {
	return apierrs.IsNotFound(err), nil
}

func getDeploymentStatus(d *v1.Deployment) corev1.ConditionStatus {
	for _, dc := range d.Status.Conditions {
		if dc.Type == "Available" {
			return dc.Status
		}
	}
	return "unknown"
}
