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
package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"knative.dev/pkg/apis"
)

func TestKnativeServingGroupVersionKind(t *testing.T) {
	r := &KnativeServing{}
	want := schema.GroupVersionKind{
		Group:   GroupName,
		Version: SchemaVersion,
		Kind:    Kind,
	}
	if got := r.GroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestKnativeServingStatusGetCondition(t *testing.T) {
	ks := &KnativeServingStatus{}
	if a := ks.GetCondition(InstallSucceeded); a != nil {
		t.Errorf("empty KnativeServingStatus returned %v when expected nil", a)
	}
	mc := &apis.Condition{
		Type:   InstallSucceeded,
		Status: corev1.ConditionTrue,
	}
	ks.MarkInstallSucceeded()
	if diff := cmp.Diff(mc, ks.GetCondition(InstallSucceeded), cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime")); diff != "" {
		t.Errorf("GetCondition refs diff (-want +got): %v", diff)
	}
}

func TestKnativeServingIsReady(t *testing.T) {
	cases := []struct {
		name    string
		status  KnativeServingStatus
		isReady bool
	}{{
		name:    "empty status should not be ready",
		status:  KnativeServingStatus{},
		isReady: false,
	}, {
		name: "Different condition type should not be ready",
		status: KnativeServingStatus{
			Conditions: apis.Conditions{{
				Type:   "FooCondition",
				Status: corev1.ConditionTrue,
			}},
		},
		isReady: false,
	}, {
		name: "False condition status should not be ready",
		status: KnativeServingStatus{
			Conditions: apis.Conditions{{
				Type:   KnativeServingConditionReady,
				Status: corev1.ConditionFalse,
			}},
		},
		isReady: false,
	}, {
		name: "Unknown condition status should not be ready",
		status: KnativeServingStatus{
			Conditions: apis.Conditions{{
				Type:   KnativeServingConditionReady,
				Status: corev1.ConditionUnknown,
			}},
		},
		isReady: false,
	}, {
		name: "Missing condition status should not be ready",
		status: KnativeServingStatus{
			Conditions: apis.Conditions{{
				Type: KnativeServingConditionReady,
			}},
		},
		isReady: false,
	}, {
		name: "True condition status should be ready",
		status: KnativeServingStatus{
			Conditions: apis.Conditions{{
				Type:   KnativeServingConditionReady,
				Status: corev1.ConditionTrue,
			}},
		},
		isReady: true,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if e, a := tc.isReady, tc.status.IsReady(); e != a {
				t.Errorf("Ready = %v, want: %v", a, e)
			}
		})
	}
}

func TestKnativeServingStatusIsAvailable(t *testing.T) {
	ks := &KnativeServingStatus{}
	if a := ks.GetCondition(DeploymentsAvailable); a != nil {
		t.Errorf("empty KnativeServingStatus returned %v when expected nil", a)
	}
	mc := &apis.Condition{
		Type:   DeploymentsAvailable,
		Status: corev1.ConditionTrue,
	}
	ks.MarkDeploymentsAvailable()
	if diff := cmp.Diff(mc, ks.GetCondition(DeploymentsAvailable), cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime")); diff != "" {
		t.Errorf("GetCondition refs diff (-want +got): %v", diff)
	}
}

func TestKnativeServingStatusInstallFailed(t *testing.T) {
	ks := &KnativeServingStatus{}
	mc := &apis.Condition{
		Type:   InstallSucceeded,
		Status: corev1.ConditionFalse,
		Reason:  "Error",
		Message: "Install failed with message: Installation failed",
	}
	ks.MarkInstallFailed("Installation failed")
	if diff := cmp.Diff(mc, ks.GetCondition(InstallSucceeded), cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime")); diff != "" {
		t.Errorf("GetCondition refs diff (-want +got): %v", diff)
	}
}

func TestKnativeServingStatusDeploymentsNotReady(t *testing.T) {
	ks := &KnativeServingStatus{}
	mc := &apis.Condition{
		Type:   DeploymentsAvailable,
		Status: corev1.ConditionFalse,
		Reason:  "NotReady",
		Message: "Waiting on deployments",
	}
	ks.MarkDeploymentsNotReady()
	if diff := cmp.Diff(mc, ks.GetCondition(DeploymentsAvailable), cmpopts.IgnoreFields(apis.Condition{}, "LastTransitionTime")); diff != "" {
		t.Errorf("GetCondition refs diff (-want +got): %v", diff)
	}
}
