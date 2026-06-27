package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"
)

// namespace is the top-level Prometheus namespace shared by every collector.
const namespace = "hbase"

// Prometheus subsystems, one per collector role.
const (
	subsystemJvm    = "jvm"
	subsystemServer = "server"
	subsystemRegion = "region"
)

// httpClient is shared across collectors. The HBase JMX endpoint can hang when
// a node is unresponsive, so a timeout keeps one slow scrape from stalling the
// whole exporter. (ponytail: single shared client, add per-collector tuning if
// need arises.)
var httpClient = &http.Client{Timeout: 30 * time.Second}

// metricDef is a single static metric: a descriptor plus closures that read a
// value and its label values from the decoded JMX response.
type metricDef[T any] struct {
	Type   prometheus.ValueType
	Desc   *prometheus.Desc
	Value  func(T) float64
	Labels func(T) []string
}

// jmxCollector is the shared base for the JVM / master / regionserver-server
// collectors. They all fetch one JMX bean, unmarshal beans[0] into a response
// struct, and emit a static list of const metrics. The role-specific behavior
// lives in the metric definitions and the label/value closures.
type jmxCollector[T any] struct {
	logger log.Logger
	url    *url.URL
	bean   string // the Hadoop:service=HBase,name=... JMX query value

	up                              prometheus.Gauge
	totalScrapes, jsonParseFailures prometheus.Counter

	metrics []metricDef[T]
}

// newJmxCollector wires the shared instrumentation metrics for a subsystem.
func newJmxCollector[T any](logger log.Logger, u *url.URL, subsystem, bean string, metrics []metricDef[T]) *jmxCollector[T] {
	return &jmxCollector[T]{
		logger: logger,
		url:    u,
		bean:   bean,
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: prometheus.BuildFQName(namespace, subsystem, "up"),
			Help: "Was the last scrape of the HBase JMX endpoint successful.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, subsystem, "total_scrapes"),
			Help: "Current total HBase JMX scrapes.",
		}),
		jsonParseFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Name: prometheus.BuildFQName(namespace, subsystem, "json_parse_failures"),
			Help: "Number of errors while parsing JSON.",
		}),
		metrics: metrics,
	}
}

func (c *jmxCollector[T]) Describe(ch chan<- *prometheus.Desc) {
	for _, m := range c.metrics {
		ch <- m.Desc
	}
	ch <- c.up.Desc()
	ch <- c.totalScrapes.Desc()
	ch <- c.jsonParseFailures.Desc()
}

func (c *jmxCollector[T]) Collect(ch chan<- prometheus.Metric) {
	c.totalScrapes.Inc()
	defer func() {
		ch <- c.up
		ch <- c.totalScrapes
		ch <- c.jsonParseFailures
	}()

	resp, err := c.fetchAndDecode()
	if err != nil {
		c.up.Set(0)
		_ = level.Warn(c.logger).Log("msg", "failed to fetch and decode HBase JMX", "err", err)
		return
	}
	c.up.Set(1)

	for _, m := range c.metrics {
		ch <- prometheus.MustNewConstMetric(m.Desc, m.Type, m.Value(resp), m.Labels(resp)...)
	}
}

// fetchAndDecode fetches the configured JMX bean and unmarshals beans[0].
func (c *jmxCollector[T]) fetchAndDecode() (T, error) {
	var zero T

	u := *c.url
	q := u.Query()
	q.Set("qry", c.bean)
	u.RawQuery = q.Encode()

	res, err := httpClient.Get(u.String())
	if err != nil {
		return zero, fmt.Errorf("failed to fetch JMX from %s: %w", u.String(), err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("HTTP request to %s failed with code %d", u.String(), res.StatusCode)
	}

	bts, err := io.ReadAll(res.Body)
	if err != nil {
		c.jsonParseFailures.Inc()
		return zero, err
	}

	beans := gjson.GetBytes(bts, "beans").Array()
	if len(beans) == 0 {
		c.jsonParseFailures.Inc()
		return zero, fmt.Errorf("no beans in JMX response from %s", u.String())
	}

	var out T
	if err := json.Unmarshal([]byte(beans[0].Raw), &out); err != nil {
		c.jsonParseFailures.Inc()
		return zero, err
	}
	return out, nil
}

// hostRoleLabels builds the standard (host, role) label values, lower-casing
// the role to normalize JMX tag.Context across collectors.
func hostRoleLabels(host, role string) []string {
	return []string{host, strings.ToLower(role)}
}
