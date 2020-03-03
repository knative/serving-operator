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
package main

import (
	"flag"
	"log"

	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"
	"knative.dev/serving-operator/pkg/reconciler/knativeserving"
)

func main() {
	flag.Parse()

	cfg, err := sharedmain.GetConfig(*knativeserving.MasterURL, *knativeserving.Kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig", err)
	}
	sharedmain.WebhookMainWithConfig(signals.NewContext(), "serving_operator", cfg, knativeserving.NewController)
}
