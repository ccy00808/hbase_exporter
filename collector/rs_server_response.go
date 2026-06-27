package collector

type rsServerResponse struct {
	Host                  string  `json:"tag.Hostname"`
	Role                  string  `json:"tag.Context"`
	MemStoreSize          int     `json:"memStoreSize"`
	RegionCount           int     `json:"regionCount"`
	StoreCount            int     `json:"storeCount"`
	StoreFileCount        int     `json:"storeFileCount"`
	StoreFileSize         int     `json:"storeFileSize"`
	TotalRequestCount     int     `json:"totalRequestCount"`
	ReadRequestCount      int     `json:"readRequestCount"`
	WriteRequestCount     int     `json:"writeRequestCount"`
	SplitQueueLength      int     `json:"splitQueueLength"`
	CompactionQueueLength int     `json:"compactionQueueLength"`
	FlushQueueLength      int     `json:"flushQueueLength"`
	BlockCacheCount       int     `json:"blockCacheCount"`
	BlockCacheSize        int     `json:"blockCacheSize"`
	BlockCacheHitCount    int     `json:"blockCacheHitCount"`
	BlockCacheMissCount   int     `json:"blockCacheMissCount"`
	BlockCountHitPercent  float64 `json:"blockCountHitPercent"`
	SlowAppendCount       int     `json:"slowAppendCount"`
	SlowDeleteCount       int     `json:"slowDeleteCount"`
	SlowGetCount          int     `json:"slowGetCount"`
	SlowPutCount          int     `json:"slowPutCount"`
	SlowIncrementCount    int     `json:"slowIncrementCount"`
}
