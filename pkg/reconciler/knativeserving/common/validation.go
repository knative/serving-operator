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
package common

import (
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/rogpeppe/go-internal/semver"
	v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/version"
	opversion "knative.dev/serving-operator/version"
)

const (
	// IstioDependency name
	istioDependency     = "istio"
	istioNamespace      = "istio-system"
	istioPilotContainer = "istio-proxy"
	// CertManagerDependency name
	certManagerDependency = "certmanager"
	certManagerNamespace  = "cert-manager"
)

type depMeta struct {
	// dependent
	name string
	// in which dependent installed
	namespace string
	// deployments installed in the namespace
	deploy []string
}

var depMetas = map[string]depMeta{
	istioDependency: {
		name:      istioDependency,
		namespace: istioNamespace,
		deploy: []string{
			"istio-ingressgateway",
			"cluster-local-gateway",
		},
	},
	certManagerDependency: {
		name:      certManagerDependency,
		namespace: certManagerNamespace,
		deploy: []string{
			"cert-manager",
		},
	},
}

func checkDependencyVersion(depname, installedVersion, expectedVersion string, log *zap.SugaredLogger) error {
	log.Debugf("Installed istio version %s", installedVersion)
	ver := semver.Canonical(installedVersion)
	if semver.Compare(expectedVersion, installedVersion) == 1 {
		return fmt.Errorf("%q version %q is not compatible, need at least %q",
			depname, ver, expectedVersion)
	}
	log.Debugf("%s installed version %s matched with expected version %s", depname, installedVersion, expectedVersion)
	return nil
}

func checkDeploymentVersion(deploy *v1beta1.Deployment, depName string, log *zap.SugaredLogger) error {
	var ver, expect string
	for _, con := range deploy.Spec.Template.Spec.Containers {
		if (con.Name == deploy.Name && depName == certManagerDependency) ||
			(depName == istioDependency && con.Name == istioPilotContainer) {
			img := strings.Split(con.Image, ":")
			if len(img) != 2 {
				log.Errorf("failed to get version from %s", con.Image)
				return fmt.Errorf("failed to get %q version from %q", depName, con.Image)
			}
			ver = img[1]
			if !strings.HasPrefix(ver, "v") {
				ver = "v" + ver
			}
			break
		}
	}

	if depName == certManagerDependency {
		expect = opversion.CertManager
	} else if depName == istioDependency {
		expect = opversion.Istio
	}

	return checkDependencyVersion(depName, ver, expect, log)
}

func validate(client kubernetes.Interface, t *depMeta, log *zap.SugaredLogger) error {
	// check namespace created
	_, err := client.CoreV1().Namespaces().Get(t.namespace, metav1.GetOptions{})
	if err != nil {
		log.Error(t.namespace + " doesn't exist")
		return err
	}

	versionValid := false

	// check deployments installed successuflly and version satisfied with requirements
	for _, deployName := range t.deploy {
		deploy, err := client.ExtensionsV1beta1().Deployments(t.namespace).Get(deployName, metav1.GetOptions{})
		if err != nil {
			log.Errorf("deployment %s doesn't exist", deployName)
			return err
		}
		// check installed version
		if !versionValid {
			err = checkDeploymentVersion(deploy, t.name, log)
			if err != nil {
				return err
			}
			versionValid = true
		}

		if deploy.Status.Replicas != deploy.Status.ReadyReplicas {
			log.Errorf("deployment %s is not available", deployName)
			return fmt.Errorf("deployment %v is not available", deployName)
		}

		log.Debugf("deployment %s is available", deployName)
	}

	log.Infof("%s validation succeeded", t.name)
	return nil
}

// ValidateDependency Firstly scan deps specified in CR to determin whether they're supported
// or not. Then perform dependency validation
func ValidateDependency(kclient kubernetes.Interface, deps []string, log *zap.SugaredLogger) error {
	log.Debugf("Dependency validation started")
	// check k8s version
	if err := version.CheckMinimumVersion(kclient.Discovery()); err != nil {
		log.Errorf("Failed to get k8s version")
		return err
	}
	log.Infof("k8s version matched")

	var tasks []*depMeta
	log.Debugf("Dependency %s to be Validated", deps)
	for _, dep := range deps {
		if val, ok := depMetas[dep]; ok {
			tasks = append(tasks, &val)
		} else {
			log.Warnf("ignore unknown dependency %s", dep)
		}
	}

	for _, t := range tasks {
		err := validate(kclient, t, log)
		if err != nil {
			return err
		}
	}

	return nil
}
