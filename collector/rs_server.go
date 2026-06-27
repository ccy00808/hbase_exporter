package collector

import (
	"net/url"

	"github.com/go-kit/kit/log"
)

// RsServer is the Prometheus collector for the HBase RegionServer,sub=Server
// bean. Only registered on regionserver-scraping instances.
type RsServer struct {
	*jmxCollector[rsServerResponse]
}

func NewRsServer(logger log.Logger, u *url.URL) *RsServer {
	labels := func(r rsServerResponse) []string { return hostRoleLabels(r.Host, r.Role) }
	metrics := []metricDef[rsServerResponse]{
		gauge(subsystemServer, "mem_store_size", "Sum of memstore sizes across all regions in bytes.", labels, func(r rsServerResponse) float64 { return float64(r.MemStoreSize) }),
		gauge(subsystemServer, "region_count", "Number of regions served.", labels, func(r rsServerResponse) float64 { return float64(r.RegionCount) }),
		gauge(subsystemServer, "store_count", "Number of stores.", labels, func(r rsServerResponse) float64 { return float64(r.StoreCount) }),
		gauge(subsystemServer, "store_file_count", "Number of store files.", labels, func(r rsServerResponse) float64 { return float64(r.StoreFileCount) }),
		gauge(subsystemServer, "store_file_size", "Aggregate size of store files in bytes.", labels, func(r rsServerResponse) float64 { return float64(r.StoreFileSize) }),
		gauge(subsystemServer, "total_request_count", "Total number of requests.", labels, func(r rsServerResponse) float64 { return float64(r.TotalRequestCount) }),
		gauge(subsystemServer, "read_request_count", "Number of read requests.", labels, func(r rsServerResponse) float64 { return float64(r.ReadRequestCount) }),
		gauge(subsystemServer, "write_request_count", "Number of write requests.", labels, func(r rsServerResponse) float64 { return float64(r.WriteRequestCount) }),
		gauge(subsystemServer, "split_queue_length", "Length of the split queue.", labels, func(r rsServerResponse) float64 { return float64(r.SplitQueueLength) }),
		gauge(subsystemServer, "compaction_queue_length", "Length of the compaction queue.", labels, func(r rsServerResponse) float64 { return float64(r.CompactionQueueLength) }),
		gauge(subsystemServer, "flush_queue_length", "Length of the flush queue.", labels, func(r rsServerResponse) float64 { return float64(r.FlushQueueLength) }),
		gauge(subsystemServer, "block_cache_count", "Number of block cache entries.", labels, func(r rsServerResponse) float64 { return float64(r.BlockCacheCount) }),
		gauge(subsystemServer, "block_cache_size", "Size of the block cache in bytes.", labels, func(r rsServerResponse) float64 { return float64(r.BlockCacheSize) }),
		gauge(subsystemServer, "block_cache_hit_count", "Number of block cache hits.", labels, func(r rsServerResponse) float64 { return float64(r.BlockCacheHitCount) }),
		gauge(subsystemServer, "block_cache_miss_count", "Number of block cache misses.", labels, func(r rsServerResponse) float64 { return float64(r.BlockCacheMissCount) }),
		gauge(subsystemServer, "block_count_hit_percent", "Block cache hit ratio percentage.", labels, func(r rsServerResponse) float64 { return r.BlockCountHitPercent }),
		gauge(subsystemServer, "slow_append_count", "Number of slow append operations.", labels, func(r rsServerResponse) float64 { return float64(r.SlowAppendCount) }),
		gauge(subsystemServer, "slow_delete_count", "Number of slow delete operations.", labels, func(r rsServerResponse) float64 { return float64(r.SlowDeleteCount) }),
		gauge(subsystemServer, "slow_get_count", "Number of slow get operations.", labels, func(r rsServerResponse) float64 { return float64(r.SlowGetCount) }),
		gauge(subsystemServer, "slow_put_count", "Number of slow put operations.", labels, func(r rsServerResponse) float64 { return float64(r.SlowPutCount) }),
		gauge(subsystemServer, "slow_increment_count", "Number of slow increment operations.", labels, func(r rsServerResponse) float64 { return float64(r.SlowIncrementCount) }),
	}
	return &RsServer{newJmxCollector(logger, u, subsystemServer, "Hadoop:service=HBase,name=RegionServer,sub=Server", metrics)}
}
