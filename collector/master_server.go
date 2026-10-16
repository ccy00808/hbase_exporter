package collector

import (
	"net/url"
	"strings"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

// MasterServer exports the seven numeric Master JMX MBeans configured by the
// JMX Prometheus Java agent. Each MBean is queried directly with `qry`, so we
// do not download and parse the complete /jmx document on every scrape.
//
// The Java agent translates an attribute such as
// AssignmentManager.AssignTime_99.9th_percentile into
// hbase_master_assignment_manager_assign_time_99_9th_percentile. This
// collector deliberately uses the same conversion and the same `instance`
// label as the external-exporter compatibility configuration.
type MasterServer struct {
	logger   log.Logger
	url      *url.URL
	instance string

	up                              prometheus.Gauge
	totalScrapes, jsonParseFailures prometheus.Counter

	mu   sync.Mutex
	jmxs []dynamicMetricPoint
}

var masterMBeans = []masterMBean{
	{jmxSub: "AssignmentManager", metricSubsystem: "assignment_manager"},
	{jmxSub: "Balancer", metricSubsystem: "balancer"},
	{jmxSub: "FileSystem", metricSubsystem: "filesystem"},
	{jmxSub: "IPC", metricSubsystem: "ipc"},
	{jmxSub: "Procedure", metricSubsystem: "procedure"},
	{jmxSub: "Quotas", metricSubsystem: "quotas"},
	{jmxSub: "Server", metricSubsystem: "server"},
}

type masterMBean struct {
	jmxSub          string
	metricSubsystem string
}

func NewMasterServer(logger log.Logger, u *url.URL, instance string) *MasterServer {
	return &MasterServer{
		logger:   logger,
		url:      u,
		instance: instance,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hbase_master_up",
			Help: "Whether every configured HBase Master JMX MBean was scraped successfully.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "hbase_master_total_scrapes",
			Help: "Total HBase Master scrape attempts.",
		}),
		jsonParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "hbase_master_json_parse_failures",
			Help: "Number of errors while parsing HBase Master JMX JSON.",
		}),
		jmxs: make([]dynamicMetricPoint, 0, 384),
	}
}

// Master MBean attributes vary slightly by HBase release. An unchecked
// collector lets a compatible numeric attribute appear without requiring a
// binary rebuild, while non-numeric JMX metadata is ignored.
func (m *MasterServer) Describe(ch chan<- *prometheus.Desc) {}

func (m *MasterServer) Collect(ch chan<- prometheus.Metric) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalScrapes.Inc()
	defer func() {
		ch <- m.up
		ch <- m.totalScrapes
		ch <- m.jsonParseFailures
	}()

	if err := m.fetch(); err != nil {
		m.up.Set(0)
		_ = level.Warn(m.logger).Log("msg", "failed to fetch HBase Master JMX", "err", err)
		return
	}
	m.up.Set(1)
	for _, point := range m.jmxs {
		ch <- prometheus.MustNewConstMetric(point.desc, prometheus.UntypedValue, point.value, point.labels...)
	}
}

func (m *MasterServer) fetch() error {
	m.jmxs = m.jmxs[:0]
	for _, mbean := range masterMBeans {
		bean, err := fetchJMXBean(m.url, "Hadoop:service=HBase,name=Master,sub="+mbean.jmxSub)
		if err != nil {
			return err
		}
		for key, value := range bean {
			if !masterNumericAttribute(key, value) {
				continue
			}
			m.jmxs = append(m.jmxs, dynamicMetricPoint{
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "master_"+mbean.metricSubsystem, jmxFieldName(key)),
					"HBase Master "+mbean.jmxSub+" JMX metric "+key+".",
					[]string{"instance"}, nil,
				),
				value:  value.Float(),
				labels: []string{m.instance},
			})
		}
	}
	return nil
}

func masterNumericAttribute(key string, value gjson.Result) bool {
	// The Java-agent configuration exports normal numeric attributes only. It
	// does not expose JMX tags/metadata or Balancer's hbase:ensemble map. The
	// remaining exclusions are numeric attributes absent from the supplied
	// Java-agent /metrics reference and are intentionally kept out so the
	// external exporter has the same metric family set.
	if value.Type != gjson.Number || strings.Contains(key, ":") {
		return false
	}
	_, excluded := excludedMasterAttributes[key]
	return !excluded
}

var excludedMasterAttributes = map[string]struct{}{
	"exceptions.callDropped":                {},
	"exceptions.callTimedOut":               {},
	"exceptions.otherExceptions":            {},
	"exceptions.requestTooBig":              {},
	"ProcessCallTime_TimeRangeCount_3-10":   {},
	"RequestSize_SizeRangeCount_1000-10000": {},
	"TotalCallTime_TimeRangeCount_3-10":     {},
}
