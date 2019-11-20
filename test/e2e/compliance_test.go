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
	"knative.dev/serving-operator/test"
	"testing"
)

func TestComplianceSuite(t *testing.T) {
	suite := ComplianceSuite()

	if len(suite) <= 0 {
		t.Error("There should be some tests that are exported as compliance suite")
	}
}

var runState map[string]int

const exampleJira = "XXXX-1234"

func TestSkipOfSpecificPartOfTestSuite(t *testing.T) {
	suite := exampleComplianceSuite()

	runState = map[string]int{
		"alpha": 0,
		"beta":  0,
		"gamma": 0,
	}

	test.
		NewContext(t).
		WithOverride(test.Skipf("TestParent/beta", "Skip due to %v", exampleJira)).
		RunSuite(suite)

	if runState["alpha"] != 1 {
		t.Error("Alpha should be executed just once")
	}
	if runState["beta"] != 0 {
		t.Error("Beta should be skipped, but wasn't")
	}
	if runState["gamma"] != 1 {
		t.Error("Gamma should be executed just once")
	}
}

func exampleComplianceSuite() []test.Specification {
	return []test.Specification{
		test.NewContextualSpec("TestParent", testParent),
	}
}

func testParent(ctx *test.Context) {
	r := ctx.Runner()

	r.Run("alpha", func(t *testing.T) {
		t.Log("Alpha is OK")
		runState["alpha"]++
	})

	r.Run("beta", func(t *testing.T) {
		runState["beta"]++
		t.Errorf("Beta is failing, because %v", exampleJira)
	})

	r.Run("gamma", func(t *testing.T) {
		t.Log("Gamma is OK")
		runState["gamma"]++
	})
}
