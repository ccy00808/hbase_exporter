package collector

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-kit/kit/log"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// mockJMX serves a fixed JMX payload for the requested qry filter.
func mockJMX(t *testing.T, payload string) *url.URL {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestRsRegionCollectsCorrectMetricNames(t *testing.T) {
	// A RegionServer,sub=Regions bean with two region metrics for one region.
	payload := `{"beans":[{
		"tag.Hostname":"rs1",
		"tag.Context":"sub",
		"Namespace_n1_table_t1_region_r1_metric_storeCount":7,
		"Namespace_n1_table_t1_region_r1_metric_readRequestCount":42
	}]}`
	u := mockJMX(t, payload)

	c := NewRsRegion(log.NewNopLogger(), u)

	// Expected metric names — must carry the "region" subsystem (this was the
	// pre-refactor bug: they came out as hbase_store_count, missing region).
	expected := `
# HELP hbase_region_read_request_count HBase region metric read_request_count.
# TYPE hbase_region_read_request_count gauge
hbase_region_read_request_count{host="rs1",hregion="r1",htable="t1",namespace="n1",role="sub"} 42
# HELP hbase_region_store_count HBase region metric store_count.
# TYPE hbase_region_store_count gauge
hbase_region_store_count{host="rs1",hregion="r1",htable="t1",namespace="n1",role="sub"} 7
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"hbase_region_read_request_count", "hbase_region_store_count"); err != nil {
		t.Fatalf("metric mismatch: %v", err)
	}

	// Up indicator must be present after a successful scrape.
	if got := testutil.ToFloat64(c.up); got != 1 {
		t.Errorf("expected hbase_region_up=1, got %v", got)
	}
}
