package v1alpha1

import (
	"github.com/knative/pkg/apis"
)

var installCondSet = apis.NewLivingConditionSet(
	InstallDeploymentsAvailable,
	InstallSucceeded,
)

// GetConditions implements apis.ConditionsAccessor
func (s *InstallStatus) GetConditions() apis.Conditions {
	return s.Conditions
}

// SetConditions implements apis.ConditionsAccessor
func (s *InstallStatus) SetConditions(c apis.Conditions) {
	s.Conditions = c
}

func (is *InstallStatus) IsReady() bool {
	return installCondSet.Manage(is).IsHappy()
}

func (is *InstallStatus) IsInstalled() bool {
	return is.GetCondition(InstallSucceeded).IsTrue()
}

func (is *InstallStatus) IsAvailable() bool {
	return is.GetCondition(InstallDeploymentsAvailable).IsTrue()
}

func (is *InstallStatus) IsDeploying() bool {
	return is.IsInstalled() && !is.IsAvailable()
}

func (is *InstallStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return installCondSet.Manage(is).GetCondition(t)
}

func (is *InstallStatus) InitializeConditions() {
	installCondSet.Manage(is).InitializeConditions()
}

func (is *InstallStatus) MarkInstallFailed(msg string) {
	installCondSet.Manage(is).MarkFalse(
		InstallSucceeded,
		"Error",
		"Install failed with message: %s", msg)
}

func (is *InstallStatus) MarkInstallSucceeded() {
	installCondSet.Manage(is).MarkTrue(InstallSucceeded)
}

func (is *InstallStatus) MarkDeploymentsAvailable() {
	installCondSet.Manage(is).MarkTrue(InstallDeploymentsAvailable)
}

func (is *InstallStatus) MarkDeploymentsNotReady() {
	installCondSet.Manage(is).MarkFalse(
		InstallDeploymentsAvailable,
		"NotReady",
		"Waiting on deployments")
}
