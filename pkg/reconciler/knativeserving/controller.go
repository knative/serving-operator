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
	"github.com/knative/serving-operator/pkg/controller/knativeserving/common"
	"k8s.io/client-go/tools/cache"
	"os"
	"path/filepath"

	mf "github.com/jcrossley3/manifestival"
	"github.com/knative/serving-operator/pkg/apis/serving/v1alpha1"
	vs1 "github.com/knative/serving-operator/pkg/client/listers/serving/v1alpha1"
	knativeServinginformer "github.com/knative/serving-operator/pkg/client/injection/informers/serving/v1alpha1/knativeserving"
	rbase "github.com/knative/serving-operator/pkg/reconciler"
	"knative.dev/pkg/configmap"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"knative.dev/pkg/controller"
	deploymentinformer "knative.dev/pkg/injection/informers/kubeinformers/appsv1/deployment"
	serviceinformer "knative.dev/pkg/injection/informers/kubeinformers/corev1/service"
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

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	knativeServingInformer := knativeServinginformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	serviceInformer := serviceinformer.Get(ctx)
	koDataDir := os.Getenv("KO_DATA_PATH")
	config, _ := mf.NewManifest(filepath.Join(koDataDir, "knative-serving/"), *recursive, common.ClusterConfig)

	c := &Reconciler{
		Base:                     rbase.NewBase(ctx, controllerAgentName, cmw),
		knativeServingLister:     knativeServingInformer.Lister(),
		deploymentLister:         deploymentInformer.Lister(),
		serviceLister:            serviceInformer.Lister(),
		config:                   config,
	}

	ensureKnativeServing(c.knativeServingLister, config)
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

func ensureKnativeServing(knativeServingLister vs1.KnativeServingLister, config mf.Manifest) (err error) {
	koDataDir := os.Getenv("KO_DATA_PATH")
	const path = "serving_v1alpha1_knativeserving_cr.yaml"
	//instance := &v1alpha1.KnativeServing{}
	//key := client.ObjectKey{Namespace: operand, Name: operand}
	if _, err := knativeServingLister.KnativeServings(operand).Get(operand); err != nil {
	//if err = c.Get(context.TODO(), key, instance); err != nil {
		var manifest mf.Manifest
		manifest, err = mf.NewManifest(filepath.Join(koDataDir, path), false, common.ClusterConfig)
		if err == nil {
			// create namespace
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
