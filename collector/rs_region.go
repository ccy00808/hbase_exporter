package collector

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"unicode"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tidwall/gjson"

	"github.com/daiwei233/hbase_exporter/utils"
)

// RsRegion exports every numeric dynamic field in the RegionServer Regions
// bean using the same metric and label convention as the JMX Java agent.
type RsRegion struct {
	logger   log.Logger
	url      *url.URL
	instance string

	up                              prometheus.Gauge
	totalScrapes, jsonParseFailures prometheus.Counter

	mu   sync.Mutex
	jmxs []dynamicMetricPoint
}

type dynamicMetricPoint struct {
	desc   *prometheus.Desc
	value  float64
	labels []string
}

// errNoJMXBeans indicates a successful JMX HTTP response whose queried MBean
// is not present on this daemon. Standby HMasters legitimately omit some
// active-only MBeans such as AssignmentManager.
var errNoJMXBeans = errors.New("no JMX beans in response")

func NewRsRegion(logger log.Logger, u *url.URL, instance string) *RsRegion {
	return &RsRegion{
		logger:   logger,
		url:      u,
		instance: instance,
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
		jmxs: make([]dynamicMetricPoint, 0, 128),
	}
}

// Region MBean fields are dynamic. Sending no descriptors intentionally makes
// this an unchecked collector, which allows newly added HBase fields to be
// exposed without code changes.
func (r *RsRegion) Describe(ch chan<- *prometheus.Desc) {}

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
		ch <- prometheus.MustNewConstMetric(p.desc, prometheus.GaugeValue, p.value, p.labels...)
	}
}

func (r *RsRegion) fetch() error {
	r.jmxs = r.jmxs[:0]
	beans, err := fetchJMXBean(r.url, "Hadoop:service=HBase,name=RegionServer,sub=Regions")
	if err != nil {
		return err
	}
	for key, value := range beans {
		if !strings.HasPrefix(key, "Namespace_") || value.Type != gjson.Number {
			continue
		}
		parts, err := utils.SplitHBaseRegionStr(key)
		if err != nil {
			r.jsonParseFailures.Inc()
			_ = level.Debug(r.logger).Log("msg", "skipping unparsable region JMX key", "key", key, "err", err)
			continue
		}
		namespace, table, regionID, field := parts[0], parts[1], parts[2], parts[3]
		r.jmxs = append(r.jmxs, dynamicMetricPoint{
			desc:   dynamicDesc("region", field, []string{"instance", "namespace", "table", "regionid"}),
			value:  value.Float(),
			labels: []string{r.instance, namespace, table, regionID},
		})
	}
	return nil
}

func fetchJMXBean(base *url.URL, bean string) (map[string]gjson.Result, error) {
	u := *base
	query := u.Query()
	query.Set("qry", bean)
	u.RawQuery = query.Encode()

	response, err := httpClient.Get(u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JMX from %s: %w", u.String(), err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request to %s failed with code %d", u.String(), response.StatusCode)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	beans := gjson.GetBytes(body, "beans").Array()
	if len(beans) == 0 {
		return nil, fmt.Errorf("%w from %s", errNoJMXBeans, u.String())
	}
	return beans[0].Map(), nil
}

func dynamicDesc(scope, field string, labels []string) *prometheus.Desc {
	name := prometheus.BuildFQName(namespace, "regionserver_"+scope, jmxFieldName(field))
	return prometheus.NewDesc(name, "HBase RegionServer "+scope+" metric "+field+".", labels, nil)
}

// jmxFieldName follows the document's lowercase output convention and turns
// camel case (and separators such as dots or dashes) into Prometheus snake
// case: majorCompactionTime_99.9th_percentile becomes
// major_compaction_time_99_9th_percentile.
func jmxFieldName(field string) string {
	// HBase writes the plural WALs as an all-capital acronym followed by a
	// lower-case plural suffix. The JMX agent emits `wals`, not `wa_ls`.
	field = strings.ReplaceAll(field, "WALs", "Wals")
	// The Java agent treats the following mixed acronym suffixes as one word.
	// Keep its established metric spellings (hfiles, kbs, walcount, walsize)
	// instead of inserting an underscore within the acronym.
	field = strings.NewReplacer(
		"HFiles", "Hfiles",
		"HFile", "Hfile",
		"KBs", "Kbs",
		"WALCount", "Walcount",
		"WALSize", "Walsize",
	).Replace(field)

	var out []rune
	runes := []rune(field)
	lastUnderscore := false
	for i, current := range runes {
		if !unicode.IsLetter(current) && !unicode.IsDigit(current) {
			if len(out) > 0 && !lastUnderscore {
				out = append(out, '_')
				lastUnderscore = true
			}
			continue
		}
		if unicode.IsUpper(current) && len(out) > 0 && !lastUnderscore {
			previous := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(previous) || unicode.IsDigit(previous) || (unicode.IsUpper(previous) && nextLower) {
				out = append(out, '_')
			}
		}
		out = append(out, unicode.ToLower(current))
		lastUnderscore = false
	}
	return strings.Trim(string(out), "_")
}
