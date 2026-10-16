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

	c := NewRsRegion(log.NewNopLogger(), u, "10.38.82.195:7004")

	// Names and labels match the JMX Prometheus Java agent mapping document.
	expected := `
# HELP hbase_regionserver_region_read_request_count HBase RegionServer region metric readRequestCount.
# TYPE hbase_regionserver_region_read_request_count gauge
hbase_regionserver_region_read_request_count{instance="10.38.82.195:7004",namespace="n1",regionid="r1",table="t1"} 42
# HELP hbase_regionserver_region_store_count HBase RegionServer region metric storeCount.
# TYPE hbase_regionserver_region_store_count gauge
hbase_regionserver_region_store_count{instance="10.38.82.195:7004",namespace="n1",regionid="r1",table="t1"} 7
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"hbase_regionserver_region_read_request_count", "hbase_regionserver_region_store_count"); err != nil {
		t.Fatalf("metric mismatch: %v", err)
	}

	// Up indicator must be present after a successful scrape.
	if got := testutil.ToFloat64(c.up); got != 1 {
		t.Errorf("expected hbase_region_up=1, got %v", got)
	}
}

func TestRsTableCollectsAgentCompatibleMetrics(t *testing.T) {
	payload := `{"beans":[{
        "Namespace_default_table_user_tag_metric_storeFileSize":1048576,
        "Namespace_default_table_user_tag_metric_majorCompactionTime_99.9th_percentile":12
    }]}`
	c := NewRsTable(log.NewNopLogger(), mockJMX(t, payload), "10.38.82.195:7004")
	expected := `
# HELP hbase_regionserver_table_major_compaction_time_99_9th_percentile HBase RegionServer table metric majorCompactionTime_99.9th_percentile.
# TYPE hbase_regionserver_table_major_compaction_time_99_9th_percentile gauge
hbase_regionserver_table_major_compaction_time_99_9th_percentile{instance="10.38.82.195:7004",namespace="default",table="user_tag"} 12
# HELP hbase_regionserver_table_store_file_size HBase RegionServer table metric storeFileSize.
# TYPE hbase_regionserver_table_store_file_size gauge
hbase_regionserver_table_store_file_size{instance="10.38.82.195:7004",namespace="default",table="user_tag"} 1.048576e+06
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"hbase_regionserver_table_major_compaction_time_99_9th_percentile",
		"hbase_regionserver_table_store_file_size"); err != nil {
		t.Fatalf("metric mismatch: %v", err)
	}
}

func TestJMXFieldName(t *testing.T) {
	for input, want := range map[string]string{
		"storeFileSize":                         "store_file_size",
		"majorCompactionTime_99.9th_percentile": "major_compaction_time_99_9th_percentile",
		"RPCQueueTime":                          "rpc_queue_time",
	} {
		if got := jmxFieldName(input); got != want {
			t.Errorf("jmxFieldName(%q) = %q, want %q", input, got, want)
		}
	}
}
