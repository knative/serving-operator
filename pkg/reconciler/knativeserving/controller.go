/*
Copyright 2019 The Knative Authors.
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

package knativeserving

import (
	"context"
	"flag"
	"fmt"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/injection/clients/kubeclient"
	"os"
	"path/filepath"

	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	mf "github.com/jcrossley3/manifestival"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/restmapper"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	deploymentinformer "knative.dev/pkg/injection/informers/kubeinformers/appsv1/deployment"
	serviceinformer "knative.dev/pkg/injection/informers/kubeinformers/corev1/service"
	"knative.dev/serving-operator/pkg/apis/serving/v1alpha1"
	knativeServinginformer "knative.dev/serving-operator/pkg/client/injection/informers/serving/v1alpha1/knativeserving"
	vs1 "knative.dev/serving-operator/pkg/client/listers/serving/v1alpha1"
	rbase "knative.dev/serving-operator/pkg/reconciler"
	"knative.dev/serving-operator/pkg/reconciler/knativeserving/common"
)

const (
	controllerAgentName = "knativeserving-controller"
	operand = "knative-serving"
)

var (
	recursive = flag.Bool("recursive", false,
		"If filename is a directory, process all manifests recursively")
	log = logf.Log.WithName("controller_knativeserving")
)

var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	knativeServingInformer := knativeServinginformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	serviceInformer := serviceinformer.Get(ctx)

	c := &Reconciler{
		Base:                     rbase.NewBase(ctx, controllerAgentName, cmw),
		knativeServingLister:     knativeServingInformer.Lister(),
		deploymentLister:         deploymentInformer.Lister(),
		serviceLister:            serviceInformer.Lister(),
	}

	mgr, err := getManager()
	if err != nil {
		c.Logger.Error(err, "Error geting the manager")
		os.Exit(1)
	}

	koDataDir := os.Getenv("KO_DATA_PATH")
	config, err := mf.NewManifest(filepath.Join(koDataDir, "knative-serving/"), *recursive, mgr.GetClient())
	if err != nil {
		c.Logger.Error(err, "Error creating the Manifest for knative-serving")
		os.Exit(1)
	}

	ensureKnativeServing(c.knativeServingLister, config, mgr.GetClient())

	c.client = mgr.GetClient()
	c.config = config

	c.scheme = mgr.GetScheme()
	c.kubeClientSet = kubeclient.Get(ctx)
	c.dynamicClientSet = dynamicclient.Get(ctx)
	reconcileKnativeServing := ReconcileKnativeServing{client: mgr.GetClient(), scheme: mgr.GetScheme(),
		clientConfig: common.ClusterConfig}
	err = reconcileKnativeServing.InjectClient(mgr.GetClient())
	if err != nil {
		log.Error(err, "Failed to inject the client")
		os.Exit(1)
	}
	c.reconcileKnativeServing = reconcileKnativeServing

	impl := controller.NewImpl(c, c.Logger, ReconcilerName)

	c.Logger.Info("Setting up event handlers for %s", ReconcilerName)

	knativeServingInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("KnativeServing")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("KnativeServing")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}

func ensureKnativeServing(knativeServingLister vs1.KnativeServingLister, config mf.Manifest, client client.Client) (err error) {
	koDataDir := os.Getenv("KO_DATA_PATH")
	const path = "serving_v1alpha1_knativeserving_cr.yaml"
	if _, err := knativeServingLister.KnativeServings(operand).Get(operand); err != nil {
		var manifest mf.Manifest
		manifest, err = mf.NewManifest(filepath.Join(koDataDir, path), false, client)
		if err == nil {
			err = manifest.Apply(&config.Resources[0])
		}
		if err == nil {
			err = manifest.Transform(mf.InjectNamespace(operand))
		}
		if err == nil {
			err = manifest.ApplyAll()
		}
	}
	return
}

func getManager() (manager.Manager, error) {
	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		return nil, err
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(common.ClusterConfig, manager.Options{
		Namespace:          namespace,
		MapperProvider:     restmapper.NewDynamicRESTMapper,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		return nil, err
	}
	return mgr, nil
}
