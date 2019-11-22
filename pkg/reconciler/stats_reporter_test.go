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

package reconciler

import (
	"testing"

	"go.opencensus.io/tag"
	"go.opencensus.io/stats/view"
	"knative.dev/pkg/metrics/metricstest"
)

const (
	reconcilerMockName = "mock_reconciler"
	testKey            = "test/key"
)

func TestNewStatsReporter(t *testing.T) {
	r, err := NewStatsReporter(reconcilerMockName)
	if err != nil {
		t.Errorf("Failed to create reporter: %v", err)
	}

	m := tag.FromContext(r.(*reporter).ctx)
	v, ok := m.Value(reconcilerTagKey)
	if !ok {
		t.Fatalf("Expected tag %q", reconcilerTagKey)
	}
	if v != reconcilerMockName {
		t.Fatalf("Expected %q for tag %q, got %q", reconcilerMockName, reconcilerTagKey, v)
	}
}

func TestReportKnativeServingChange(t *testing.T) {
	r, _ := NewStatsReporter(reconcilerMockName)
	wantTags := map[string]string{
		reconcilerTagKey.Name():     reconcilerMockName,
		knativeServingTagKey.Name(): testKey,
		changeTagKey.Name():         "creation",
	}
	countWas := int64(0)
	if d, err := view.RetrieveData(knativeServingChangeCountName); err == nil && len(d) == 1 {
		countWas = d[0].Data.(*view.CountData).Value
	}

	if err := r.ReportKnativeServingChange(testKey, "creation"); err != nil {
		t.Error(err)
	}

	metricstest.CheckCountData(t, knativeServingChangeCountName, wantTags, countWas+1)
}
