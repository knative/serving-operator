package install

import (
	"github.com/openshift-knative/knative-serving-operator/pkg/controller/install/openshift"
)

func init() {
	platformPreInstallFuncs = append(platformPreInstallFuncs, openshift.EnsureMaistra)
	platformPostInstallFuncs = append(platformPostInstallFuncs, openshift.EnsureOpenshiftIngress)
	platformTransformFuncs = append(platformTransformFuncs, openshift.Configure)
}
