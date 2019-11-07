/*
Copyright 2018 The Knative Authors

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
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"knative.dev/pkg/metrics"
)

const (
	// reconcileCountN is the number of successful reconcile operations.
	reconcileCountN = "knative_operator_reconcile_count"
	// reconcileLatencyN is the time it takes for a sucessful reconcile operation.
	reconcileLatencyN = "knative_operator_reconcile_latency"
)

var (
	reconcileCountStat = stats.Int64(
		reconcileCountN,
		"Number of reconcile operations",
		stats.UnitNone)
	reconcileLatencyStat = stats.Int64(
		reconcileLatencyN,
		"Latency of reconcile operations",
		stats.UnitMilliseconds)

	// Create the tag keys that will be used to add tags to our measurements.
	// Tag keys must conform to the restrictions described in
	// go.opencensus.io/tag/validate.go. Currently those restrictions are:
	// - length between 1 and 255 inclusive
	// - characters are printable US-ASCII
	reconcilerTagKey = tag.MustNewKey("reconciler")
	keyTagKey        = tag.MustNewKey("key")
)

func init() {
	// Create views to see our measurements. This can return an error if
	// a previously-registered view has the same name with a different value.
	// View name defaults to the measure name if unspecified.
	if err := view.Register(
		&view.View{
			Description: "Number of reconcile operations",
			Measure:     reconcileCountStat,
			Aggregation: view.Count(),
			TagKeys:     []tag.Key{reconcilerTagKey, keyTagKey},
		},
		&view.View{
			Description: "Latency of reconcile operations",
			Measure:     reconcileLatencyStat,
			Aggregation: view.LastValue(),
			TagKeys:     []tag.Key{reconcilerTagKey, keyTagKey},
		},
	); err != nil {
		panic(err)
	}
}

// StatsReporter defines the interface for sending metrics
type StatsReporter interface {
	// ReportReconcile reports the count and latency metrics for a reconcile operation
	ReportReconcile(resourceNamespace, resourceName string, duration time.Duration) error
}

// Reporter holds cached metric objects to report metrics
type reporter struct {
	reconciler string
	ctx        context.Context
}

// srKey is used to associate StatsReporters with contexts.
type srKey struct{}

// WithStatsReporter attaches the given StatsReporter to the provided context
// in the returned context.
func WithStatsReporter(ctx context.Context, sr StatsReporter) context.Context {
	return context.WithValue(ctx, srKey{}, sr)
}

// GetStatsReporter attempts to look up the StatsReporter on a given context.
// It may return null if none is found.
func GetStatsReporter(ctx context.Context) StatsReporter {
	untyped := ctx.Value(srKey{})
	if untyped == nil {
		return nil
	}
	return untyped.(StatsReporter)
}

// NewStatsReporter creates a reporter that collects and reports metrics
func NewStatsReporter(reconciler string) (StatsReporter, error) {
	// Reconciler tag is static. Create a context containing that and cache it.
	ctx, err := tag.New(
		context.Background(),
		tag.Insert(reconcilerTagKey, reconciler))
	if err != nil {
		return nil, err
	}

	return &reporter{reconciler: reconciler, ctx: ctx}, nil
}

// ReportReconcile reports the count and latency metrics for a reconcile operation
func (r *reporter) ReportReconcile(resourceNamespace, resourceName string, duration time.Duration) error {
	key := fmt.Sprintf("%s/%s", resourceNamespace, resourceName)
	ctx, err := tag.New(
		context.Background(),
		tag.Insert(reconcilerTagKey, r.reconciler),
		tag.Insert(keyTagKey, key))
	if err != nil {
		return err
	}

	metrics.Record(ctx, reconcileCountStat.M(1))
	metrics.Record(ctx, reconcileLatencyStat.M(duration.Milliseconds()))
	return nil
}
