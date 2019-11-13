package common

import (
	"errors"
	"fmt"

	mf "github.com/jcrossley3/manifestival"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/scheme"
	servingv1alpha1 "knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
)

const (
	customCertsEnvName    = "SSL_CERT_DIR"
	customCertsMountPath  = "/custom-certs"
	customCertsNamePrefix = "custom-certs-"
)

func CustomCertsTransform(instance *servingv1alpha1.KnativeServing, log *zap.SugaredLogger) mf.Transformer {
	empty := servingv1alpha1.CustomCerts{}
	return func(u *unstructured.Unstructured) error {
		if instance.Spec.ControllerCustomCerts == empty {
			return nil
		}
		if u.GetKind() == "Deployment" && u.GetName() == "controller" {
			certs := instance.Spec.ControllerCustomCerts
			deployment := &appsv1.Deployment{}
			if err := scheme.Scheme.Convert(u, deployment, nil); err != nil {
				return err
			}
			if err := configureCustomCerts(deployment, certs); err != nil {
				return err
			}
			if err := scheme.Scheme.Convert(deployment, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

func configureCustomCerts(deployment *appsv1.Deployment, certs servingv1alpha1.CustomCerts) error {
	source := v1.VolumeSource{}
	switch certs.Type {
	case "ConfigMap":
		source.ConfigMap = &v1.ConfigMapVolumeSource{
			LocalObjectReference: v1.LocalObjectReference{
				Name: certs.Name,
			},
		}
	case "Secret":
		source.Secret = &v1.SecretVolumeSource{
			SecretName: certs.Name,
		}
	default:
		return errors.New(fmt.Sprintf("Unknown CustomCerts type: %s", certs.Type))
	}

	name := customCertsNamePrefix + certs.Name
	if name == customCertsNamePrefix {
		return errors.New(fmt.Sprintf("CustomCerts name for %s is required", certs.Type))
	}
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, v1.Volume{
		Name:         name,
		VolumeSource: source,
	})

	containers := deployment.Spec.Template.Spec.Containers
	containers[0].VolumeMounts = append(containers[0].VolumeMounts, v1.VolumeMount{
		Name:      name,
		MountPath: customCertsMountPath,
	})
	containers[0].Env = append(containers[0].Env, v1.EnvVar{
		Name:  customCertsEnvName,
		Value: customCertsMountPath,
	})
	return nil
}
