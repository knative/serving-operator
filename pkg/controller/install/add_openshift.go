package install

import (
	"github.com/openshift-knative/knative-serving-operator/pkg/controller/install/openshift"
)

func init() {
	platformPrereqFuncs = append(platformPrereqFuncs, openshift.EnsureMaistra)
	platformTransformFuncs = append(platformTransformFuncs, openshift.Configure)
}
