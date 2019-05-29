package knativeserving

import (
	"github.com/openshift-knative/knative-serving-operator/pkg/controller/knativeserving/openshift"
)

func init() {
	platforms = append(platforms, openshift.Configure)
}
