package collector

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"

	"github.com/daiwei233/hbase_exporter/utils"
)

// regionLabelNames is the per-region label set: host/role plus the region's
// namespace, table and region id.
var regionLabelNames = []string{"host", "role", "namespace", "htable", "hregion"}

// regionMetric maps a JMX field suffix (the trailing "<metric>" segment of a
// "Namespace_..._metric_<metric>" key) to the Prometheus metric name emitted.
var regionMetric = map[string]string{
	"readRequestCount":         "read_request_count",
	"writeRequestCount":        "write_request_count",
	"storeCount":               "store_count",
	"storeFileCount":           "store_file_count",
	"memStoreSize":             "mem_store_size",
	"storeFileSize":            "store_file_size",
	"compactionsCompletedCount": "compactions_completed_count",
	"numBytesCompactedCount":   "num_bytes_compacted_count",
	"numFilesCompactedCount":   "num_files_compacted_count",
	"compactionsFailedCount":   "compactions_failed_count",
	"totalRequestCount":        "total_request_count",
}

// RsRegion is the Prometheus collector for the HBase RegionServer,sub=Regions
// bean. Region metrics have dynamic per-region label sets, so metrics are
// emitted via freshly-built const metrics with the region labels attached.
type RsRegion struct {
	logger log.Logger
	url    *url.URL

	up                              prometheus.Gauge
	totalScrapes, jsonParseFailures prometheus.Counter

	descs map[string]*prometheus.Desc // exporter metric name -> Desc

	mu    sync.Mutex
	jmxs  []regionMetricPoint
}

type regionMetricPoint struct {
	metric string // exporter metric name
	value  float64
	labels []string // host, role, namespace, table, region
}

func NewRsRegion(logger log.Logger, u *url.URL) *RsRegion {
	descs := make(map[string]*prometheus.Desc, len(regionMetric))
	for _, name := range regionMetric {
		descs[name] = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystemRegion, name),
			fmt.Sprintf("HBase region metric %s.", name),
			regionLabelNames, nil,
		)
	}
	return &RsRegion{
		logger: logger,
		url:    u,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: prometheus.BuildFQName(namespace, subsystemRegion, "up"),
			Help: "Was the last scrape of the HBase region JMX endpoint successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, subsystemRegion, "total_scrapes"),
			Help: "Current total HBase region JMX scrapes.",
		}),
		jsonParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, subsystemRegion, "json_parse_failures"),
			Help: "Number of errors while parsing JSON.",
		}),
		descs: descs,
		jmxs:  make([]regionMetricPoint, 0, 128),
	}
}

func (r *RsRegion) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range r.descs {
		ch <- d
	}
	ch <- r.up.Desc()
	ch <- r.totalScrapes.Desc()
	ch <- r.jsonParseFailures.Desc()
}

func (r *RsRegion) Collect(ch chan<- prometheus.Metric) {
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
		_ = level.Warn(r.logger).Log("msg", "failed to fetch and decode HBase region JMX", "err", err)
		return
	}
	r.up.Set(1)

	for _, p := range r.jmxs {
		desc, ok := r.descs[p.metric]
		if !ok {
			continue
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, p.value, p.labels...)
	}
}

// fetch scrapes the Regions bean and parses all per-region metric keys into
// points ready to emit.
func (r *RsRegion) fetch() error {
	r.jmxs = r.jmxs[:0]

	u := *r.url
	q := u.Query()
	q.Set("qry", "Hadoop:service=HBase,name=RegionServer,sub=Regions")
	u.RawQuery = q.Encode()

	res, err := httpClient.Get(u.String())
	if err != nil {
		return fmt.Errorf("failed to fetch JMX from %s: %w", u.String(), err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP request to %s failed with code %d", u.String(), res.StatusCode)
	}

	bts, err := io.ReadAll(res.Body)
	if err != nil {
		r.jsonParseFailures.Inc()
		return err
	}

	beans := gjson.GetBytes(bts, "beans").Array()
	if len(beans) == 0 {
		r.jsonParseFailures.Inc()
		return fmt.Errorf("no beans in JMX response from %s", u.String())
	}
	data := beans[0].Map()

	host := data["tag.Hostname"].String()
	role := data["tag.Context"].String()

	for k, v := range data {
		// Region metric keys look like Namespace_n1_table_t1_region_r1_metric_storeCount.
		if !strings.HasPrefix(k, "Namespace") {
			continue
		}
		parts, err := utils.SplitHBaseRegionStr(k)
		if err != nil {
			r.jsonParseFailures.Inc()
			_ = level.Debug(r.logger).Log("msg", "skipping unparsable region JMX key", "key", k, "err", err)
			continue
		}
		ns, table, region, field := parts[0], parts[1], parts[2], parts[3]
		name, ok := regionMetric[field]
		if !ok {
			continue // unknown metric field, ignore rather than fail
		}
		r.jmxs = append(r.jmxs, regionMetricPoint{
			metric: name,
			value:  v.Float(),
			labels: []string{host, role, ns, table, region},
		})
	}
	return nil
}
