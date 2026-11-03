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

func TestRsTableLatenciesCollectsTableScopedHistograms(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("qry"); got != "Hadoop:service=HBase,name=RegionServer,sub=TableLatencies" {
			t.Fatalf("unexpected JMX query: %q", got)
		}
		_, _ = w.Write([]byte(`{"beans":[{
            "Namespace_default_table_user_tag_metric_getTime_mean":4.5,
            "Namespace_default_table_user_tag_metric_putTime_95th_percentile":12
        }]}`))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	c := NewRsTableLatencies(log.NewNopLogger(), u, "10.38.81.3:7004")
	expected := `
# HELP hbase_regionserver_table_latency_get_time_mean HBase RegionServer table_latency metric getTime_mean.
# TYPE hbase_regionserver_table_latency_get_time_mean gauge
hbase_regionserver_table_latency_get_time_mean{instance="10.38.81.3:7004",namespace="default",table="user_tag"} 4.5
# HELP hbase_regionserver_table_latency_put_time_95th_percentile HBase RegionServer table_latency metric putTime_95th_percentile.
# TYPE hbase_regionserver_table_latency_put_time_95th_percentile gauge
hbase_regionserver_table_latency_put_time_95th_percentile{instance="10.38.81.3:7004",namespace="default",table="user_tag"} 12
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"hbase_regionserver_table_latency_get_time_mean",
		"hbase_regionserver_table_latency_put_time_95th_percentile"); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(c.up); got != 1 {
		t.Fatalf("expected hbase_regionserver_table_latency_up=1, got %v", got)
	}
}
