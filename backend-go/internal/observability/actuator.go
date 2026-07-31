package observability

import (
	"net/http"
	"sort"
	"strings"

	dto "github.com/prometheus/client_model/go"
)

type actuatorMeasurement struct {
	Statistic string  `json:"statistic"`
	Value     float64 `json:"value"`
}

type actuatorTag struct {
	Tag    string   `json:"tag"`
	Values []string `json:"values"`
}

func (m *Metrics) ActuatorNames(w http.ResponseWriter, _ *http.Request) {
	families, err := m.Registry.Gather()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": 500})
		return
	}
	names := make([]string, 0, len(families))
	for _, family := range families {
		names = append(names, actuatorName(family.GetName()))
	}
	sort.Strings(names)
	writeJSON(w, http.StatusOK, map[string]any{"names": names})
}

func (m *Metrics) ActuatorMetric(w http.ResponseWriter, r *http.Request) {
	requested := r.PathValue("metricName")
	families, err := m.Registry.Gather()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": 500})
		return
	}
	for _, family := range families {
		if actuatorName(family.GetName()) == requested {
			writeJSON(w, http.StatusOK, map[string]any{
				"name": requested, "description": family.GetHelp(), "baseUnit": nil,
				"measurements": measurements(family), "availableTags": availableTags(family),
			})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]any{"message": "No meter with name '" + requested + "'", "status": 404})
}

func actuatorName(prometheusName string) string {
	name := strings.TrimSuffix(prometheusName, "_total")
	name = strings.TrimSuffix(name, "_seconds")
	return strings.ReplaceAll(name, "_", ".")
}

func measurements(family *dto.MetricFamily) []actuatorMeasurement {
	var count, total, value float64
	for _, metric := range family.Metric {
		switch family.GetType() {
		case dto.MetricType_COUNTER:
			value += metric.GetCounter().GetValue()
		case dto.MetricType_GAUGE:
			value += metric.GetGauge().GetValue()
		case dto.MetricType_HISTOGRAM:
			count += float64(metric.GetHistogram().GetSampleCount())
			total += metric.GetHistogram().GetSampleSum()
		case dto.MetricType_SUMMARY:
			count += float64(metric.GetSummary().GetSampleCount())
			total += metric.GetSummary().GetSampleSum()
		case dto.MetricType_UNTYPED:
			value += metric.GetUntyped().GetValue()
		}
	}
	if family.GetType() == dto.MetricType_HISTOGRAM || family.GetType() == dto.MetricType_SUMMARY {
		return []actuatorMeasurement{{Statistic: "COUNT", Value: count}, {Statistic: "TOTAL", Value: total}}
	}
	return []actuatorMeasurement{{Statistic: "VALUE", Value: value}}
}

func availableTags(family *dto.MetricFamily) []actuatorTag {
	values := map[string]map[string]struct{}{}
	for _, metric := range family.Metric {
		for _, label := range metric.Label {
			if values[label.GetName()] == nil {
				values[label.GetName()] = map[string]struct{}{}
			}
			values[label.GetName()][label.GetValue()] = struct{}{}
		}
	}
	tags := make([]actuatorTag, 0, len(values))
	for name, entries := range values {
		items := make([]string, 0, len(entries))
		for value := range entries {
			items = append(items, value)
		}
		sort.Strings(items)
		tags = append(tags, actuatorTag{Tag: name, Values: items})
	}
	sort.Slice(tags, func(left, right int) bool { return tags[left].Tag < tags[right].Tag })
	return tags
}
