// Copyright 2019 OpenTelemetry Authors
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

package carbonexporter

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestSanitizeTagKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want string
	}{
		{
			name: "no_changes",
			key:  "a valid tag key",
			want: "a valid tag key",
		},
		{
			name: "remove_tag_set",
			key:  "a" + tagKeyValueSeparator + "c",
			want: "a" + string(sanitizedRune) + "c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTagKey(tt.key)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSanitizeTagValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "no_changes",
			value: "a valid tag value",
			want:  "a valid tag value",
		},
		{
			name:  "replace_tilde",
			value: "a~c",
			want:  "a" + string(sanitizedRune) + "c",
		},
		{
			name:  "replace_semicol",
			value: "a;c",
			want:  "a" + string(sanitizedRune) + "c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeTagValue(tt.value)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildPath(t *testing.T) {
	tests := []struct {
		name       string
		attributes pcommon.Map
		want       string
	}{
		{
			name: "happy_path",
			attributes: func() pcommon.Map {
				attr := pcommon.NewMap()
				attr.PutStr("key0", "val0")
				return attr
			}(),
			want: "happy_path;key0=val0",
		},
		{
			name: "empty_value",
			attributes: func() pcommon.Map {
				attr := pcommon.NewMap()
				attr.PutStr("k0", "")
				attr.PutStr("k1", "v1")
				return attr
			}(),
			want: "empty_value;k0=" + tagValueEmptyPlaceholder + ";k1=v1",
		},
		{
			name: "int_value",
			attributes: func() pcommon.Map {
				attr := pcommon.NewMap()
				attr.PutInt("k", 1)
				return attr
			}(),
			want: "int_value;k=1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPath(tt.name, tt.attributes)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToPlaintext(t *testing.T) {
	expectedTagsStr := ";k0=v0;k1=v1"

	unixSecs := int64(1574092046)
	expectedUnixSecsStr := strconv.FormatInt(unixSecs, 10)
	unixNSecs := int64(11 * time.Millisecond)
	tsUnix := time.Unix(unixSecs, unixNSecs)

	doubleVal := 1234.5678
	expectedDobuleValStr := strconv.FormatFloat(doubleVal, 'g', -1, 64)
	int64Val := int64(123)
	expectedInt64ValStr := "123"

	distributionCount := uint64(16)
	distributionSum := float64(34.56)
	distributionBounds := []float64{1.5, 2, 4}
	distributionCounts := []uint64{4, 2, 3, 7}

	summaryCount := uint64(11)
	summarySum := float64(111)
	summaryQuantiles := []float64{90, 95, 99, 99.9}
	summaryQuantileValues := []float64{100, 6, 4, 1}
	tests := []struct {
		name                       string
		metricsDataFn              func() pmetric.Metrics
		wantLines                  []string
		wantNumConvertedTimeseries int
		wantNumDroppedTimeseries   int
	}{
		{
			name: "no_dims",
			metricsDataFn: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				ms.AppendEmpty().SetName("gauge_double_no_dims")
				dps1 := ms.At(0).SetEmptyGauge().DataPoints()
				dps1.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dps1.At(0).SetDoubleValue(doubleVal)
				ms.AppendEmpty().SetName("gauge_int_no_dims")
				dps2 := ms.At(1).SetEmptyGauge().DataPoints()
				dps2.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dps2.At(0).SetIntValue(int64Val)

				ms.AppendEmpty().SetName("cumulative_double_no_dims")
				ms.At(2).SetEmptySum().SetIsMonotonic(true)
				dps3 := ms.At(2).Sum().DataPoints()
				dps3.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dps3.At(0).SetDoubleValue(doubleVal)
				ms.AppendEmpty().SetName("cumulative_int_no_dims")
				ms.At(3).SetEmptySum().SetIsMonotonic(true)
				dps4 := ms.At(3).Sum().DataPoints()
				dps4.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dps4.At(0).SetIntValue(int64Val)
				return md

			},
			wantLines: []string{
				"gauge_double_no_dims " + expectedDobuleValStr + " " + expectedUnixSecsStr,
				"gauge_int_no_dims " + expectedInt64ValStr + " " + expectedUnixSecsStr,
				"cumulative_double_no_dims " + expectedDobuleValStr + " " + expectedUnixSecsStr,
				"cumulative_int_no_dims " + expectedInt64ValStr + " " + expectedUnixSecsStr,
			},
			wantNumConvertedTimeseries: 4,
		},
		{
			name: "with_dims",
			metricsDataFn: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				ms.AppendEmpty().SetName("gauge_double_with_dims")
				dps1 := ms.At(0).SetEmptyGauge().DataPoints()
				dps1.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dps1.At(0).Attributes().PutStr("k0", "v0")
				dps1.At(0).Attributes().PutStr("k1", "v1")
				dps1.At(0).SetDoubleValue(doubleVal)
				ms.AppendEmpty().SetName("gauge_int_with_dims")
				dps2 := ms.At(1).SetEmptyGauge().DataPoints()
				dps2.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dps2.At(0).Attributes().PutStr("k0", "v0")
				dps2.At(0).Attributes().PutStr("k1", "v1")
				dps2.At(0).SetIntValue(int64Val)

				ms.AppendEmpty().SetName("cumulative_double_with_dims")
				ms.At(2).SetEmptySum().SetIsMonotonic(true)
				dps3 := ms.At(2).Sum().DataPoints()
				dps3.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dps3.At(0).Attributes().PutStr("k0", "v0")
				dps3.At(0).Attributes().PutStr("k1", "v1")
				dps3.At(0).SetDoubleValue(doubleVal)
				ms.AppendEmpty().SetName("cumulative_int_with_dims")
				ms.At(3).SetEmptySum().SetIsMonotonic(true)
				dps4 := ms.At(3).Sum().DataPoints()
				dps4.AppendEmpty().SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dps4.At(0).Attributes().PutStr("k0", "v0")
				dps4.At(0).Attributes().PutStr("k1", "v1")
				dps4.At(0).SetIntValue(int64Val)
				return md
			},
			wantLines: []string{
				"gauge_double_with_dims" + expectedTagsStr + " " + expectedDobuleValStr + " " + expectedUnixSecsStr,
				"gauge_int_with_dims" + expectedTagsStr + " " + expectedInt64ValStr + " " + expectedUnixSecsStr,
				"cumulative_double_with_dims" + expectedTagsStr + " " + expectedDobuleValStr + " " + expectedUnixSecsStr,
				"cumulative_int_with_dims" + expectedTagsStr + " " + expectedInt64ValStr + " " + expectedUnixSecsStr,
			},
			wantNumConvertedTimeseries: 4,
		},
		{
			name: "distributions",
			metricsDataFn: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				ms.AppendEmpty().SetName("distrib")
				ms.At(0).SetEmptyHistogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				dp := ms.At(0).SetEmptyHistogram().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.Attributes().FromRaw(map[string]interface{}{"k0": "v0", "k1": "v1"})
				dp.Attributes().Sort() // ensures result order
				dp.SetCount(distributionCount)
				dp.SetSum(distributionSum)
				dp.ExplicitBounds().FromRaw(distributionBounds)
				dp.BucketCounts().FromRaw(distributionCounts)
				return md
			},
			wantLines: expectedDistributionLines(
				"distrib", expectedTagsStr, expectedUnixSecsStr,
				distributionSum,
				distributionCount,
				distributionBounds,
				distributionCounts),
			wantNumConvertedTimeseries: 1,
		},
		{
			name: "summary",
			metricsDataFn: func() pmetric.Metrics {
				md := pmetric.NewMetrics()
				ms := md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics()
				ms.AppendEmpty().SetName("summary")
				dp := ms.At(0).SetEmptySummary().DataPoints().AppendEmpty()
				dp.SetTimestamp(pcommon.NewTimestampFromTime(tsUnix))
				dp.Attributes().FromRaw(map[string]interface{}{"k0": "v0", "k1": "v1"})
				dp.Attributes().Sort() // ensures result order
				dp.SetCount(summaryCount)
				dp.SetSum(summarySum)
				for i := range summaryQuantiles {
					qv := dp.QuantileValues().AppendEmpty()
					qv.SetQuantile(summaryQuantiles[i] / 100)
					qv.SetValue(summaryQuantileValues[i])
				}
				return md
			},
			wantLines: expectedSummaryLines(
				"summary", expectedTagsStr, expectedUnixSecsStr,
				summarySum,
				summaryCount,
				summaryQuantiles,
				summaryQuantileValues),
			wantNumConvertedTimeseries: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLines := metricDataToPlaintext(tt.metricsDataFn())
			got := strings.Split(gotLines, "\n")
			got = got[:len(got)-1]
			assert.Equal(t, tt.wantLines, got)
		})
	}
}

func expectedDistributionLines(
	metricName, tags, timestampStr string,
	sum float64,
	count uint64,
	bounds []float64,
	counts []uint64,
) []string {
	lines := []string{
		metricName + ".count" + tags + " " + formatInt64(int64(count)) + " " + timestampStr,
		metricName + tags + " " + formatFloatForLabel(sum) + " " + timestampStr,
	}

	for i, bound := range bounds {
		lines = append(lines,
			metricName+".bucket"+tags+";upper_bound="+formatFloatForLabel(bound)+" "+formatInt64(int64(counts[i]))+" "+timestampStr)
	}
	lines = append(lines,
		metricName+".bucket"+tags+";upper_bound=inf "+formatInt64(int64(counts[len(bounds)]))+" "+timestampStr)

	return lines
}

func expectedSummaryLines(
	metricName, tags, timestampStr string,
	sum float64,
	count uint64,
	summaryQuantiles []float64,
	summaryQuantileValues []float64,
) []string {
	lines := []string{
		metricName + ".count" + tags + " " + formatInt64(int64(count)) + " " + timestampStr,
		metricName + tags + " " + formatFloatForValue(sum) + " " + timestampStr,
	}

	for i := range summaryQuantiles {
		lines = append(lines,
			metricName+".quantile"+tags+";quantile="+formatFloatForLabel(summaryQuantiles[i])+" "+formatFloatForValue(summaryQuantileValues[i])+" "+timestampStr)
	}

	return lines
}
