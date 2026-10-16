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

func TestMasterServerSkipsMissingOptionalMBean(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("qry"), "AssignmentManager") {
			_, _ = w.Write([]byte(`{"beans":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"beans":[{"averageLoad":3.5}]}`))
	}))
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := NewMasterServer(log.NewNopLogger(), u, "10.38.82.195:7005")
	expected := `
# HELP hbase_master_server_average_load HBase Master Server JMX metric averageLoad.
# TYPE hbase_master_server_average_load untyped
hbase_master_server_average_load{instance="10.38.82.195:7005"} 3.5
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected), "hbase_master_server_average_load"); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(c.up); got != 1 {
		t.Fatalf("hbase_master_up = %v, want 1", got)
	}
}

func TestMasterServerUsesAgentNameAndInstanceLabel(t *testing.T) {
	payload := `{"beans":[{
        "averageLoad":3.5,
        "numRegionServers":7,
        "numDeadRegionServers":1,
        "tag.isActiveMaster":"true"
    }]}`
	c := NewMasterServer(log.NewNopLogger(), mockJMX(t, payload), "10.38.82.195:7005")
	expected := `
	# HELP hbase_master_server_average_load HBase Master Server JMX metric averageLoad.
	# TYPE hbase_master_server_average_load untyped
hbase_master_server_average_load{instance="10.38.82.195:7005"} 3.5
	# HELP hbase_master_server_num_region_servers HBase Master Server JMX metric numRegionServers.
	# TYPE hbase_master_server_num_region_servers untyped
hbase_master_server_num_region_servers{instance="10.38.82.195:7005"} 7
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"hbase_master_server_average_load", "hbase_master_server_num_region_servers"); err != nil {
		t.Fatal(err)
	}
}

func TestRegionServerUsesAgentNameAndInstanceLabel(t *testing.T) {
	payload := `{"beans":[{
        "memStoreSize":1024,
        "readRequestCount":42
    }]}`
	c := NewRsServer(log.NewNopLogger(), mockJMX(t, payload), "10.38.82.195:7004")
	expected := `
	# HELP hbase_regionserver_server_mem_store_size HBase RegionServer Server JMX metric memStoreSize.
	# TYPE hbase_regionserver_server_mem_store_size untyped
hbase_regionserver_server_mem_store_size{instance="10.38.82.195:7004"} 1024
# HELP hbase_regionserver_server_read_request_count HBase RegionServer Server JMX metric readRequestCount.
# TYPE hbase_regionserver_server_read_request_count untyped
hbase_regionserver_server_read_request_count{instance="10.38.82.195:7004"} 42
`
	if err := testutil.CollectAndCompare(c, strings.NewReader(expected),
		"hbase_regionserver_server_mem_store_size", "hbase_regionserver_server_read_request_count"); err != nil {
		t.Fatal(err)
	}
}

func TestJMXFieldNameMatchesAgentAcronymHandling(t *testing.T) {
	tests := map[string]string{
		"numMasterWALs":                 "num_master_wals",
		"shippedHFiles":                 "shipped_hfiles",
		"shippedKBs":                    "shipped_kbs",
		"mutationsWithoutWALCount":      "mutations_without_walcount",
		"mutationsWithoutWALSize":       "mutations_without_walsize",
		"sizeOfHFileRefsQueue":          "size_of_hfile_refs_queue",
	}
	for input, want := range tests {
		if got := jmxFieldName(input); got != want {
			t.Fatalf("jmxFieldName(%s) = %q, want %q", input, got, want)
		}
	}
}
