// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/metrics"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottldatapoints"
)

func convertSummaryCountValToSum(stringAggTemp string, monotonic bool) (ottl.ExprFunc[ottldatapoints.TransformContext], error) {
	var aggTemp pmetric.AggregationTemporality
	switch stringAggTemp {
	case "delta":
		aggTemp = pmetric.AggregationTemporalityDelta
	case "cumulative":
		aggTemp = pmetric.AggregationTemporalityCumulative
	default:
		return nil, fmt.Errorf("unknown aggregation temporality: %s", stringAggTemp)
	}
	return func(ctx ottldatapoints.TransformContext) interface{} {
		metric := ctx.GetMetric()
		if metric.Type() != pmetric.MetricTypeSummary {
			return nil
		}

		sumMetric := ctx.GetMetrics().AppendEmpty()
		sumMetric.SetDescription(metric.Description())
		sumMetric.SetName(metric.Name() + "_count")
		sumMetric.SetUnit(metric.Unit())
		sumMetric.SetEmptySum().SetAggregationTemporality(aggTemp)
		sumMetric.Sum().SetIsMonotonic(monotonic)

		sumDps := sumMetric.Sum().DataPoints()
		dps := metric.Summary().DataPoints()
		for i := 0; i < dps.Len(); i++ {
			dp := dps.At(i)
			sumDp := sumDps.AppendEmpty()
			dp.Attributes().CopyTo(sumDp.Attributes())
			sumDp.SetIntValue(int64(dp.Count()))
			sumDp.SetStartTimestamp(dp.StartTimestamp())
			sumDp.SetTimestamp(dp.Timestamp())
		}
		return nil
	}, nil
}
