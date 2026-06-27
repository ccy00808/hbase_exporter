package collector

import (
	"net/url"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus"
)

// Jvm is the Prometheus collector for HBase JvmMetrics, scraped on both the
// master and regionserver roles.
type Jvm struct {
	*jmxCollector[jvmResponse]
}

// hostRoleLabelNames is the standard (host, role) label set shared by the JVM,
// master and regionserver-server collectors.
var hostRoleLabelNames = []string{"host", "role"}

func NewJvm(logger log.Logger, u *url.URL) *Jvm {
	labels := func(r jvmResponse) []string { return hostRoleLabels(r.Host, r.Role) }
	metrics := []metricDef[jvmResponse]{
		gauge(subsystemJvm, "mem_non_heap_used_m", "Non-heap memory used in MB.", labels, func(r jvmResponse) float64 { return r.MemNonHeapUsedM }),
		gauge(subsystemJvm, "mem_heap_used_m", "Heap memory used in MB.", labels, func(r jvmResponse) float64 { return r.MemHeapUsedM }),
		gauge(subsystemJvm, "mem_heap_max_m", "Maximum heap memory in MB.", labels, func(r jvmResponse) float64 { return r.MemHeapMaxM }),
		gauge(subsystemJvm, "mem_max_m", "Maximum memory in MB.", labels, func(r jvmResponse) float64 { return r.MemMaxM }),
		gauge(subsystemJvm, "gc_time_millis", "Time spent in GC in milliseconds.", labels, func(r jvmResponse) float64 { return float64(r.GcTimeMillis) }),
		gauge(subsystemJvm, "gc_count", "Total number of GC events.", labels, func(r jvmResponse) float64 { return float64(r.GcCount) }),
		gauge(subsystemJvm, "thread_blocked", "Number of blocked threads.", labels, func(r jvmResponse) float64 { return float64(r.ThreadsBlocked) }),
	}
	return &Jvm{newJmxCollector(logger, u, subsystemJvm, "Hadoop:service=HBase,name=JvmMetrics", metrics)}
}

// gauge builds a labelled gauge metric definition over a response type.
func gauge[T any](subsystem, name, help string, labels func(T) []string, value func(T) float64) metricDef[T] {
	return metricDef[T]{
		Type:   prometheus.GaugeValue,
		Desc:   prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, name), help, hostRoleLabelNames, nil),
		Value:  value,
		Labels: labels,
	}
}

