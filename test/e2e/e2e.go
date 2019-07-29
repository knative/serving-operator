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

package e2e

import (
	"fmt"
	"os"
	"testing"

	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"knative.dev/serving-operator/test"
	pkgTest "knative.dev/pkg/test"
)

// Logger default implementation
type Logger struct{}

// Section prints a message
func (l Logger) Section(msg string, f func()) {
	fmt.Printf("==> %s\n", msg)
	f()
}

// Debugf prints a debug message
func (l Logger) Debugf(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
}

// Fatalf prints a fatal message
func (l Logger) Fatalf(msg string, args ...interface{}) {
	fmt.Printf(msg, args...)
	os.Exit(1)
}

func Setup(t *testing.T) *test.Clients {
	clients, err := test.NewClients(
		pkgTest.Flags.Kubeconfig,
		pkgTest.Flags.Cluster)
	if err != nil {
		t.Fatalf("Couldn't initialize clients: %v", err)
	}
	return clients
}
