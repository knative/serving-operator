package install

import (
	"github.com/openshift-knative/knative-serving-operator/pkg/controller/install/openshift"
)

func init() {
	platforms = append(platforms, openshift.Configure)
}
