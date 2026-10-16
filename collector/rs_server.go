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

// RsServer exports the fixed RegionServer MBeans included in the JMX Prometheus
// Java-agent configuration. Dynamic Tables and Regions MBeans remain separate
// collectors because they add namespace/table/regionid labels.
type RsServer struct {
	logger   log.Logger
	url      *url.URL
	instance string

	up                              prometheus.Gauge
	totalScrapes, jsonParseFailures prometheus.Counter

	mu   sync.Mutex
	jmxs []dynamicMetricPoint
}

var regionServerMBeans = []string{
	"IPC",
	"Memory",
	"Replication",
	"Server",
	"WAL",
}

func NewRsServer(logger log.Logger, u *url.URL, instance string) *RsServer {
	return &RsServer{
		logger:   logger,
		url:      u,
		instance: instance,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hbase_regionserver_up",
			Help: "Whether every configured HBase RegionServer JMX MBean was scraped successfully.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "hbase_regionserver_total_scrapes",
			Help: "Total HBase RegionServer scrape attempts.",
		}),
		jsonParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "hbase_regionserver_json_parse_failures",
			Help: "Number of errors while parsing HBase RegionServer JMX JSON.",
		}),
		jmxs: make([]dynamicMetricPoint, 0, 800),
	}
}

// RegionServer fixed MBeans vary slightly by HBase release. An unchecked
// collector keeps the compatible numeric fields available without embedding a
// per-version response struct.
func (r *RsServer) Describe(ch chan<- *prometheus.Desc) {}

func (r *RsServer) Collect(ch chan<- prometheus.Metric) {
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
		_ = level.Warn(r.logger).Log("msg", "failed to fetch HBase RegionServer JMX", "err", err)
		return
	}
	r.up.Set(1)
	for _, point := range r.jmxs {
		ch <- prometheus.MustNewConstMetric(point.desc, prometheus.UntypedValue, point.value, point.labels...)
	}
}

func (r *RsServer) fetch() error {
	r.jmxs = r.jmxs[:0]
	for _, sub := range regionServerMBeans {
		bean, err := fetchJMXBean(r.url, "Hadoop:service=HBase,name=RegionServer,sub="+sub)
		if err != nil {
			return err
		}
		for key, value := range bean {
			if value.Type != gjson.Number || strings.Contains(key, ":") {
				continue
			}
			r.jmxs = append(r.jmxs, dynamicMetricPoint{
				desc: prometheus.NewDesc(
					prometheus.BuildFQName(namespace, "regionserver_"+jmxFieldName(sub), jmxFieldName(key)),
					"HBase RegionServer "+sub+" JMX metric "+key+".",
					[]string{"instance"}, nil,
				),
				value:  value.Float(),
				labels: []string{r.instance},
			})
		}
	}
	return nil
}
