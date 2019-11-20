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
package version

import (
	"path/filepath"
	"runtime"
	"testing"

	mf "github.com/jcrossley3/manifestival"
)

func TestManifestVersionSame(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	resources, err := mf.Parse(filepath.Join(filepath.Dir(b)+"/..", "cmd/manager/kodata/knative-serving/"), false)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}

	// example: v0.10.1
	expectedLabelValue := "v" + Version

	for _, resource := range resources {
		v, found := resource.GetLabels()["serving.knative.dev/release"]
		if !found {
			// label is missing for this resource. we cannot do the check.
			continue
		}
		if v != expectedLabelValue {
			t.Errorf("Version info in manifest and operator don't match. got: %v, want: %v. Resource GVK: %v, Resource name: %v", v, expectedLabelValue,
				resource.GroupVersionKind(), resource.GetName())
		}
	}
}
