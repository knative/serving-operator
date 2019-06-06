package v1alpha1

import (
	"github.com/knative/pkg/apis"
)

var conditions = apis.NewLivingConditionSet(
	DeploymentsAvailable,
	InstallSucceeded,
)

// GetConditions implements apis.ConditionsAccessor
func (s *KnativeServingStatus) GetConditions() apis.Conditions {
	return s.Conditions
}

// SetConditions implements apis.ConditionsAccessor
func (s *KnativeServingStatus) SetConditions(c apis.Conditions) {
	s.Conditions = c
}

func (is *KnativeServingStatus) IsReady() bool {
	return conditions.Manage(is).IsHappy()
}

func (is *KnativeServingStatus) IsInstalled() bool {
	return is.GetCondition(InstallSucceeded).IsTrue()
}

func (is *KnativeServingStatus) IsAvailable() bool {
	return is.GetCondition(DeploymentsAvailable).IsTrue()
}

func (is *KnativeServingStatus) IsDeploying() bool {
	return is.IsInstalled() && !is.IsAvailable()
}

func (is *KnativeServingStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return conditions.Manage(is).GetCondition(t)
}

func (is *KnativeServingStatus) InitializeConditions() {
	conditions.Manage(is).InitializeConditions()
}

func (is *KnativeServingStatus) MarkInstallFailed(msg string) {
	conditions.Manage(is).MarkFalse(
		InstallSucceeded,
		"Error",
		"Install failed with message: %s", msg)
}

func (is *KnativeServingStatus) MarkIgnored(msg string) {
	conditions.Manage(is).MarkFalse(
		InstallSucceeded,
		"Ignored",
		"Install not attempted: %s", msg)
}

func (is *KnativeServingStatus) MarkInstallSucceeded() {
	conditions.Manage(is).MarkTrue(InstallSucceeded)
}

func (is *KnativeServingStatus) MarkDeploymentsAvailable() {
	conditions.Manage(is).MarkTrue(DeploymentsAvailable)
}

func (is *KnativeServingStatus) MarkDeploymentsNotReady() {
	conditions.Manage(is).MarkFalse(
		DeploymentsAvailable,
		"NotReady",
		"Waiting on deployments")
}
