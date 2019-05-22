package install

import (
	"github.com/openshift-knative/knative-serving-operator/pkg/controller/install/minikube"
)

func init() {
	platforms = append(platforms, minikube.Configure)
}
