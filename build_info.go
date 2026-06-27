package main

import (
	"runtime"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"
)

// newBuildInfoCollector exposes a single "<name>_build_info" gauge carrying the
// exporter's build metadata as labels — the modern replacement for the
// version.NewCollector that newer prometheus/common dropped. A constant 1.0
// value lets users select/filter on the build labels in PromQL.
func newBuildInfoCollector(name string) prometheus.Collector {
	return &buildInfoCollector{
		desc: prometheus.NewDesc(
			prometheus.BuildFQName(name, "", "build_info"),
			"A metric with a constant '1' value labelled by exporter build info.",
			[]string{"version", "revision", "goversion"}, nil,
		),
	}
}

type buildInfoCollector struct {
	desc *prometheus.Desc
}

func (c *buildInfoCollector) Describe(ch chan<- *prometheus.Desc) { ch <- c.desc }

func (c *buildInfoCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, 1,
		version.Info(), version.GetRevision(), runtime.Version())
}
