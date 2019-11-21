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
	"context"
	"fmt"
	"testing"
	"time"

	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"

	"knative.dev/pkg/metrics/metricstest"
)

const (
	reconcilerMockName    = "mock_reconciler"
	testResourceNamespace = "test_namespace"
	testResourceName      = "test_resource"
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

func TestReporter_ReportInstall(t *testing.T) {
	reporter, err := NewStatsReporter(reconcilerMockName)
	if err != nil {
		t.Errorf("Failed to create reporter: %v", err)
	}
	countWas := int64(0)
	if m := getMetric(t, InstallSuccessCountName); m != nil {
		countWas = m.Data.(*view.CountData).Value
	}
	expectedTags := map[string]string{
		keyTagKey.Name():        fmt.Sprintf("%s/%s", testResourceNamespace, testResourceName),
		reconcilerTagKey.Name(): reconcilerMockName,
	}
	
	shortTime, longTime := 1100.0, 50000.0
	if err = reporter.ReportInstallSuccess(testResourceNamespace, testResourceName, time.Duration(shortTime)*time.Millisecond); err != nil {
		t.Error(err)
	}
	if err = reporter.ReportInstallSuccess(testResourceNamespace, testResourceName, time.Duration(longTime)*time.Millisecond); err != nil {
		t.Error(err)
	}

	metricstest.CheckCountData(t, InstallSuccessCountName, expectedTags, countWas+2)
	metricstest.CheckDistributionData(t, InstallSuccessLatencyName, expectedTags, 2, shortTime, longTime)
}

func TestReporter_ReportDeployment(t *testing.T) {
	reporter, err := NewStatsReporter(reconcilerMockName)
	if err != nil {
		t.Errorf("Failed to create reporter: %v", err)
	}
	countWas := int64(0)
	if m := getMetric(t, DeploymentReadyCountName); m != nil {
		countWas = m.Data.(*view.CountData).Value
	}
	expectedTags := map[string]string{
		keyTagKey.Name():        fmt.Sprintf("%s/%s", testResourceNamespace, testResourceName),
		reconcilerTagKey.Name(): reconcilerMockName,
	}

	shortTime, longTime := 1500.0, 35000.0
	if err = reporter.ReportDeploymentReady(testResourceNamespace, testResourceName, time.Duration(shortTime)*time.Millisecond); err != nil {
		t.Error(err)
	}
	if err = reporter.ReportDeploymentReady(testResourceNamespace, testResourceName, time.Duration(longTime)*time.Millisecond); err != nil {
		t.Error(err)
	}

	metricstest.CheckCountData(t, DeploymentReadyCountName, expectedTags, countWas+2)
	metricstest.CheckDistributionData(t, DeploymentReadyLatencyName, expectedTags, 2, shortTime, longTime)
}

func getMetric(t *testing.T, metric string) *view.Row {
	t.Helper()
	rows, err := view.RetrieveData(metric)
	if err != nil {
		t.Errorf("Failed retrieving data: %v", err)
	}
	if len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func TestWithStatsReporter(t *testing.T) {
	if WithStatsReporter(context.TODO(), nil) == nil {
		t.Errorf("stats reporter reports empty context")
	}
}
