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

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

const (
	nameStr     = "__name__"
	sumStr      = "_sum"
	countStr    = "_count"
	bucketStr   = "_bucket"
	leStr       = "le"
	quantileStr = "quantile"
	pInfStr     = "+Inf"
	// maxExemplarRunes is the maximum number of UTF-8 exemplar characters
	// according to the prometheus specification
	// https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md#exemplars
	maxExemplarRunes = 128
	// Trace and Span id keys are defined as part of the spec:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification%2Fmetrics%2Fdatamodel.md#exemplars-2
	traceIDKey       = "trace_id"
	spanIDKey        = "span_id"
	infoType         = "info"
	targetMetricName = "target_info"
)

type bucketBoundsData struct {
	sig   string
	bound float64
}

// byBucketBoundsData enables the usage of sort.Sort() with a slice of bucket bounds
type byBucketBoundsData []bucketBoundsData

func (m byBucketBoundsData) Len() int           { return len(m) }
func (m byBucketBoundsData) Less(i, j int) bool { return m[i].bound < m[j].bound }
func (m byBucketBoundsData) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// ByLabelName enables the usage of sort.Sort() with a slice of labels
type ByLabelName []prompb.Label

func (a ByLabelName) Len() int           { return len(a) }
func (a ByLabelName) Less(i, j int) bool { return a[i].Name < a[j].Name }
func (a ByLabelName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// addSample finds a TimeSeries in tsMap that corresponds to the label set labels, and add sample to the TimeSeries; it
// creates a new TimeSeries in the map if not found and returns the time series signature.
// tsMap will be unmodified if either labels or sample is nil, but can still be modified if the exemplar is nil.
func addSample(tsMap map[string]*prompb.TimeSeries, sample *prompb.Sample, labels []prompb.Label,
	datatype string) string {

	if sample == nil || labels == nil || tsMap == nil {
		return ""
	}

	sig := timeSeriesSignature(datatype, &labels)
	ts, ok := tsMap[sig]

	if ok {
		ts.Samples = append(ts.Samples, *sample)
	} else {
		newTs := &prompb.TimeSeries{
			Labels:  labels,
			Samples: []prompb.Sample{*sample},
		}
		tsMap[sig] = newTs
	}

	return sig
}

// addExemplars finds a bucket bound that corresponds to the exemplars value and add the exemplar to the specific sig;
// we only add exemplars if samples are presents
// tsMap is unmodified if either of its parameters is nil and samples are nil.
func addExemplars(tsMap map[string]*prompb.TimeSeries, exemplars []prompb.Exemplar, bucketBoundsData []bucketBoundsData) {
	if tsMap == nil || bucketBoundsData == nil || exemplars == nil {
		return
	}

	sort.Sort(byBucketBoundsData(bucketBoundsData))

	for _, exemplar := range exemplars {
		addExemplar(tsMap, bucketBoundsData, exemplar)
	}
}

func addExemplar(tsMap map[string]*prompb.TimeSeries, bucketBounds []bucketBoundsData, exemplar prompb.Exemplar) {
	for _, bucketBound := range bucketBounds {
		sig := bucketBound.sig
		bound := bucketBound.bound

		_, ok := tsMap[sig]
		if ok {
			if tsMap[sig].Samples != nil {
				if exemplar.Value <= bound {
					tsMap[sig].Exemplars = append(tsMap[sig].Exemplars, exemplar)
					return
				}
			}
		}
	}
}

// timeSeries return a string signature in the form of:
//
//	TYPE-label1-value1- ...  -labelN-valueN
//
// the label slice should not contain duplicate label names; this method sorts the slice by label name before creating
// the signature.
func timeSeriesSignature(datatype string, labels *[]prompb.Label) string {
	b := strings.Builder{}
	b.WriteString(datatype)

	sort.Sort(ByLabelName(*labels))

	for _, lb := range *labels {
		b.WriteString("-")
		b.WriteString(lb.GetName())
		b.WriteString("-")
		b.WriteString(lb.GetValue())
	}

	return b.String()
}

// createAttributes creates a slice of Cortex Label with OTLP attributes and pairs of string values.
// Unpaired string value is ignored. String pairs overwrites OTLP labels if collision happens, and the overwrite is
// logged. Resultant label names are sanitized.
func createAttributes(resource pcommon.Resource, attributes pcommon.Map, externalLabels map[string]string, extras ...string) []prompb.Label {
	// map ensures no duplicate label name
	l := map[string]prompb.Label{}

	// Ensure attributes are sorted by key for consistent merging of keys which
	// collide when sanitized.
	// Sorting is done on a cloned map, as the original attributes map can read at
	// the same time in different places.
	cloneAttributes := pcommon.NewMap()
	attributes.CopyTo(cloneAttributes)
	cloneAttributes.Sort()
	cloneAttributes.Range(func(key string, value pcommon.Value) bool {
		var finalKey = prometheustranslator.NormalizeLabel(key)
		if existingLabel, alreadyExists := l[finalKey]; alreadyExists {
			existingLabel.Value = existingLabel.Value + ";" + value.AsString()
			l[finalKey] = existingLabel
		} else {
			l[finalKey] = prompb.Label{
				Name:  finalKey,
				Value: value.AsString(),
			}
		}

		return true
	})

	// Map service.name + service.namespace to job
	if serviceName, ok := resource.Attributes().Get(conventions.AttributeServiceName); ok {
		val := serviceName.AsString()
		if serviceNamespace, ok := resource.Attributes().Get(conventions.AttributeServiceNamespace); ok {
			val = fmt.Sprintf("%s/%s", serviceNamespace.AsString(), val)
		}
		l[model.JobLabel] = prompb.Label{
			Name:  model.JobLabel,
			Value: val,
		}
	}
	// Map service.instance.id to instance
	if instance, ok := resource.Attributes().Get(conventions.AttributeServiceInstanceID); ok {
		l[model.InstanceLabel] = prompb.Label{
			Name:  model.InstanceLabel,
			Value: instance.AsString(),
		}
	}
	for key, value := range externalLabels {
		// External labels have already been sanitized
		if _, alreadyExists := l[key]; alreadyExists {
			// Skip external labels if they are overridden by metric attributes
			continue
		}
		l[key] = prompb.Label{
			Name:  key,
			Value: value,
		}
	}

	for i := 0; i < len(extras); i += 2 {
		if i+1 >= len(extras) {
			break
		}
		_, found := l[extras[i]]
		if found {
			log.Println("label " + extras[i] + " is overwritten. Check if Prometheus reserved labels are used.")
		}
		// internal labels should be maintained
		name := extras[i]
		if !(len(name) > 4 && name[:2] == "__" && name[len(name)-2:] == "__") {
			name = prometheustranslator.NormalizeLabel(name)
		}
		l[name] = prompb.Label{
			Name:  name,
			Value: extras[i+1],
		}
	}

	s := make([]prompb.Label, 0, len(l))
	for _, lb := range l {
		s = append(s, lb)
	}

	return s
}

// validateMetrics returns a bool representing whether the metric has a valid type and temporality combination and a
// matching metric type and field
func validateMetrics(metric pmetric.Metric) bool {
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		return metric.Gauge().DataPoints().Len() != 0
	case pmetric.MetricTypeSum:
		return metric.Sum().DataPoints().Len() != 0 && metric.Sum().AggregationTemporality() == pmetric.AggregationTemporalityCumulative
	case pmetric.MetricTypeHistogram:
		return metric.Histogram().DataPoints().Len() != 0 && metric.Histogram().AggregationTemporality() == pmetric.AggregationTemporalityCumulative
	case pmetric.MetricTypeSummary:
		return metric.Summary().DataPoints().Len() != 0
	}
	return false
}

// addSingleNumberDataPoint converts the metric value stored in pt to a Prometheus sample, and add the sample
// to its corresponding time series in tsMap
func addSingleNumberDataPoint(pt pmetric.NumberDataPoint, resource pcommon.Resource, metric pmetric.Metric, settings Settings, tsMap map[string]*prompb.TimeSeries) {
	// create parameters for addSample
	name := prometheustranslator.BuildPromCompliantName(metric, settings.Namespace)
	labels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nameStr, name)
	sample := &prompb.Sample{
		// convert ns to ms
		Timestamp: convertTimeStamp(pt.Timestamp()),
	}
	switch pt.ValueType() {
	case pmetric.NumberDataPointValueTypeInt:
		sample.Value = float64(pt.IntValue())
	case pmetric.NumberDataPointValueTypeDouble:
		sample.Value = pt.DoubleValue()
	}
	if pt.Flags().NoRecordedValue() {
		sample.Value = math.Float64frombits(value.StaleNaN)
	}
	addSample(tsMap, sample, labels, metric.Type().String())
}

// addSingleHistogramDataPoint converts pt to 2 + min(len(ExplicitBounds), len(BucketCount)) + 1 samples. It
// ignore extra buckets if len(ExplicitBounds) > len(BucketCounts)
func addSingleHistogramDataPoint(pt pmetric.HistogramDataPoint, resource pcommon.Resource, metric pmetric.Metric, settings Settings, tsMap map[string]*prompb.TimeSeries) {
	time := convertTimeStamp(pt.Timestamp())
	// sum, count, and buckets of the histogram should append suffix to baseName
	baseName := prometheustranslator.BuildPromCompliantName(metric, settings.Namespace)

	// If the sum is unset, it indicates the _sum metric point should be
	// omitted
	if pt.HasSum() {
		// treat sum as a sample in an individual TimeSeries
		sum := &prompb.Sample{
			Value:     pt.Sum(),
			Timestamp: time,
		}
		if pt.Flags().NoRecordedValue() {
			sum.Value = math.Float64frombits(value.StaleNaN)
		}

		sumlabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nameStr, baseName+sumStr)
		addSample(tsMap, sum, sumlabels, metric.Type().String())
	}

	// treat count as a sample in an individual TimeSeries
	count := &prompb.Sample{
		Value:     float64(pt.Count()),
		Timestamp: time,
	}
	if pt.Flags().NoRecordedValue() {
		count.Value = math.Float64frombits(value.StaleNaN)
	}

	countlabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nameStr, baseName+countStr)
	addSample(tsMap, count, countlabels, metric.Type().String())

	// cumulative count for conversion to cumulative histogram
	var cumulativeCount uint64

	promExemplars := getPromExemplars(pt)

	var bucketBounds []bucketBoundsData

	// process each bound, based on histograms proto definition, # of buckets = # of explicit bounds + 1
	for i := 0; i < pt.ExplicitBounds().Len() && i < pt.BucketCounts().Len(); i++ {
		bound := pt.ExplicitBounds().At(i)
		cumulativeCount += pt.BucketCounts().At(i)
		bucket := &prompb.Sample{
			Value:     float64(cumulativeCount),
			Timestamp: time,
		}
		if pt.Flags().NoRecordedValue() {
			bucket.Value = math.Float64frombits(value.StaleNaN)
		}
		boundStr := strconv.FormatFloat(bound, 'f', -1, 64)
		labels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nameStr, baseName+bucketStr, leStr, boundStr)
		sig := addSample(tsMap, bucket, labels, metric.Type().String())

		bucketBounds = append(bucketBounds, bucketBoundsData{sig: sig, bound: bound})
	}
	// add le=+Inf bucket
	infBucket := &prompb.Sample{
		Timestamp: time,
	}
	if pt.Flags().NoRecordedValue() {
		infBucket.Value = math.Float64frombits(value.StaleNaN)
	} else {
		if pt.BucketCounts().Len() > 0 {
			cumulativeCount += pt.BucketCounts().At(pt.BucketCounts().Len() - 1)
		}
		infBucket.Value = float64(cumulativeCount)
	}
	infLabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nameStr, baseName+bucketStr, leStr, pInfStr)
	sig := addSample(tsMap, infBucket, infLabels, metric.Type().String())

	bucketBounds = append(bucketBounds, bucketBoundsData{sig: sig, bound: math.Inf(1)})
	addExemplars(tsMap, promExemplars, bucketBounds)
}

func getPromExemplars(pt pmetric.HistogramDataPoint) []prompb.Exemplar {
	var promExemplars []prompb.Exemplar

	for i := 0; i < pt.Exemplars().Len(); i++ {
		exemplar := pt.Exemplars().At(i)
		exemplarRunes := 0

		promExemplar := &prompb.Exemplar{
			Value:     exemplar.DoubleValue(),
			Timestamp: timestamp.FromTime(exemplar.Timestamp().AsTime()),
		}
		if !exemplar.TraceID().IsEmpty() {
			val := exemplar.TraceID().HexString()
			exemplarRunes += utf8.RuneCountInString(traceIDKey) + utf8.RuneCountInString(val)
			promLabel := prompb.Label{
				Name:  traceIDKey,
				Value: val,
			}
			promExemplar.Labels = append(promExemplar.Labels, promLabel)
		}
		if !exemplar.SpanID().IsEmpty() {
			val := exemplar.SpanID().HexString()
			exemplarRunes += utf8.RuneCountInString(spanIDKey) + utf8.RuneCountInString(val)
			promLabel := prompb.Label{
				Name:  spanIDKey,
				Value: val,
			}
			promExemplar.Labels = append(promExemplar.Labels, promLabel)
		}
		var labelsFromAttributes []prompb.Label

		exemplar.FilteredAttributes().Range(func(key string, value pcommon.Value) bool {
			val := value.AsString()
			exemplarRunes += utf8.RuneCountInString(key) + utf8.RuneCountInString(val)
			promLabel := prompb.Label{
				Name:  key,
				Value: val,
			}

			labelsFromAttributes = append(labelsFromAttributes, promLabel)

			return true
		})
		if exemplarRunes <= maxExemplarRunes {
			// only append filtered attributes if it does not cause exemplar
			// labels to exceed the max number of runes
			promExemplar.Labels = append(promExemplar.Labels, labelsFromAttributes...)
		}

		promExemplars = append(promExemplars, *promExemplar)
	}

	return promExemplars
}

// mostRecentTimestampInMetric returns the latest timestamp in a batch of metrics
func mostRecentTimestampInMetric(metric pmetric.Metric) pcommon.Timestamp {
	var ts pcommon.Timestamp
	// handle individual metric based on type
	switch metric.Type() {
	case pmetric.MetricTypeGauge:
		dataPoints := metric.Gauge().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = maxTimestamp(ts, dataPoints.At(x).Timestamp())
		}
	case pmetric.MetricTypeSum:
		dataPoints := metric.Sum().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = maxTimestamp(ts, dataPoints.At(x).Timestamp())
		}
	case pmetric.MetricTypeHistogram:
		dataPoints := metric.Histogram().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = maxTimestamp(ts, dataPoints.At(x).Timestamp())
		}
	case pmetric.MetricTypeSummary:
		dataPoints := metric.Summary().DataPoints()
		for x := 0; x < dataPoints.Len(); x++ {
			ts = maxTimestamp(ts, dataPoints.At(x).Timestamp())
		}
	}
	return ts
}

func maxTimestamp(a, b pcommon.Timestamp) pcommon.Timestamp {
	if a > b {
		return a
	}
	return b
}

// addSingleSummaryDataPoint converts pt to len(QuantileValues) + 2 samples.
func addSingleSummaryDataPoint(pt pmetric.SummaryDataPoint, resource pcommon.Resource, metric pmetric.Metric, settings Settings,
	tsMap map[string]*prompb.TimeSeries) {
	time := convertTimeStamp(pt.Timestamp())
	// sum and count of the summary should append suffix to baseName
	baseName := prometheustranslator.BuildPromCompliantName(metric, settings.Namespace)
	// treat sum as a sample in an individual TimeSeries
	sum := &prompb.Sample{
		Value:     pt.Sum(),
		Timestamp: time,
	}
	if pt.Flags().NoRecordedValue() {
		sum.Value = math.Float64frombits(value.StaleNaN)
	}
	sumlabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nameStr, baseName+sumStr)
	addSample(tsMap, sum, sumlabels, metric.Type().String())

	// treat count as a sample in an individual TimeSeries
	count := &prompb.Sample{
		Value:     float64(pt.Count()),
		Timestamp: time,
	}
	if pt.Flags().NoRecordedValue() {
		count.Value = math.Float64frombits(value.StaleNaN)
	}
	countlabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nameStr, baseName+countStr)
	addSample(tsMap, count, countlabels, metric.Type().String())

	// process each percentile/quantile
	for i := 0; i < pt.QuantileValues().Len(); i++ {
		qt := pt.QuantileValues().At(i)
		quantile := &prompb.Sample{
			Value:     qt.Value(),
			Timestamp: time,
		}
		if pt.Flags().NoRecordedValue() {
			quantile.Value = math.Float64frombits(value.StaleNaN)
		}
		percentileStr := strconv.FormatFloat(qt.Quantile(), 'f', -1, 64)
		qtlabels := createAttributes(resource, pt.Attributes(), settings.ExternalLabels, nameStr, baseName, quantileStr, percentileStr)
		addSample(tsMap, quantile, qtlabels, metric.Type().String())
	}
}

// addResourceTargetInfo converts the resource to the target info metric
func addResourceTargetInfo(resource pcommon.Resource, settings Settings, timestamp pcommon.Timestamp, tsMap map[string]*prompb.TimeSeries) {
	if settings.DisableTargetInfo {
		return
	}
	// Use resource attributes (other than those used for job+instance) as the
	// metric labels for the target info metric
	attributes := pcommon.NewMap()
	resource.Attributes().CopyTo(attributes)
	attributes.RemoveIf(func(k string, _ pcommon.Value) bool {
		switch k {
		case conventions.AttributeServiceName, conventions.AttributeServiceNamespace, conventions.AttributeServiceInstanceID:
			// Remove resource attributes used for job + instance
			return true
		default:
			return false
		}
	})
	if attributes.Len() == 0 {
		// If we only have job + instance, then target_info isn't useful, so don't add it.
		return
	}
	// create parameters for addSample
	name := targetMetricName
	if len(settings.Namespace) > 0 {
		name = settings.Namespace + "_" + name
	}
	labels := createAttributes(resource, attributes, settings.ExternalLabels, nameStr, name)
	sample := &prompb.Sample{
		Value: float64(1),
		// convert ns to ms
		Timestamp: convertTimeStamp(timestamp),
	}
	addSample(tsMap, sample, labels, infoType)
}

// convertTimeStamp converts OTLP timestamp in ns to timestamp in ms
func convertTimeStamp(timestamp pcommon.Timestamp) int64 {
	return timestamp.AsTime().UnixNano() / (int64(time.Millisecond) / int64(time.Nanosecond))
}
