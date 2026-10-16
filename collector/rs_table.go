package collector

import (
	"net/url"
	"strings"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"

	"github.com/daiwei233/hbase_exporter/utils"
)

// RsTable exports every numeric dynamic field in the RegionServer Tables bean
// as hbase_regionserver_table_<field>{instance,namespace,table}.
type RsTable struct {
	logger   log.Logger
	url      *url.URL
	instance string

	up                              prometheus.Gauge
	totalScrapes, jsonParseFailures prometheus.Counter

	mu   sync.Mutex
	jmxs []dynamicMetricPoint
}

func NewRsTable(logger log.Logger, u *url.URL, instance string) *RsTable {
	return &RsTable{
		logger:   logger,
		url:      u,
		instance: instance,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: prometheus.BuildFQName(namespace, "regionserver_table", "up"),
			Help: "Was the last scrape of the HBase table JMX endpoint successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, "regionserver_table", "total_scrapes"),
			Help: "Current total HBase table JMX scrapes.",
		}),
		jsonParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, "regionserver_table", "json_parse_failures"),
			Help: "Number of errors while parsing table JMX JSON.",
		}),
		jmxs: make([]dynamicMetricPoint, 0, 128),
	}
}

func (r *RsTable) Describe(ch chan<- *prometheus.Desc) {}

func (r *RsTable) Collect(ch chan<- prometheus.Metric) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.totalScrapes.Inc()
	defer func() {
		ch <- r.up
		ch <- r.totalScrapes
		ch <- r.jsonParseFailures
	}()

	if err := r.fetch(); err != nil {
		r.up.Set(0)
		_ = level.Warn(r.logger).Log("msg", "failed to fetch and decode HBase table JMX", "err", err)
		return
	}
	r.up.Set(1)
	for _, p := range r.jmxs {
		ch <- prometheus.MustNewConstMetric(p.desc, prometheus.GaugeValue, p.value, p.labels...)
	}
}

func (r *RsTable) fetch() error {
	r.jmxs = r.jmxs[:0]
	beans, err := fetchJMXBean(r.url, "Hadoop:service=HBase,name=RegionServer,sub=Tables")
	if err != nil {
		return err
	}
	for key, value := range beans {
		if !strings.HasPrefix(key, "Namespace_") || value.Type != gjson.Number {
			continue
		}
		parts, err := utils.SplitHBaseTableStr(key)
		if err != nil {
			r.jsonParseFailures.Inc()
			_ = level.Debug(r.logger).Log("msg", "skipping unparsable table JMX key", "key", key, "err", err)
			continue
		}
		namespace, table, field := parts[0], parts[1], parts[2]
		r.jmxs = append(r.jmxs, dynamicMetricPoint{
			desc:   dynamicDesc("table", field, []string{"instance", "namespace", "table"}),
			value:  value.Float(),
			labels: []string{r.instance, namespace, table},
		})
	}
	return nil
}
