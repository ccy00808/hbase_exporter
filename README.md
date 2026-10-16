# HBase Exporter

Prometheus exporter for HBase which fetches data from the HBase JMX endpoint, written in Go. Targets **HBase 1.2.x** metrics.

You can see region-level metrics too.



## Installation and Usage

For pre-built binaries see the [releases](../../releases).

### Build from source

```sh
make build
```

### Docker

```sh
docker build -t hbase_exporter .
docker run -d -p 9115:9115 hbase_exporter \
  --hbase.regionserver.uri=http://<rs-host>:60030/jmx
```

Container images for each release are published to `ghcr.io/daiwei233/hbase_exporter`.



## Configuration

Below is the command line options summary:

`hbase_exporter --help`

| Argument               | Description                                           | Default                    |
| ---------------------- | ----------------------------------------------------- | -------------------------- |
| web.listen-address     | Address to listen on for web interface and telemetry. | :9115                      |
| web.telemetry-path     | Path under which to expose metrics.                   | /metrics                   |
| hbase.master.uri       | HTTP jmx address of an HBase master node.             | http://localhost:60010/jmx |
| hbase.regionserver.uri | HTTP jmx address of an HBase regionserver node.       | http://localhost:60030/jmx |
| hbase.master           | Scrape the HBase master (instead of a regionserver).  | false                      |
| hbase.instance         | `instance` label matching the Java agent.             | JMX host + `:7005`/`:7004` |
| log.level              | Log level: debug, info, warn, error.                  | info                       |
| log.format             | Log format: json, logfmt.                             | logfmt                     |
| log.output             | Log output: stdout, stderr.                           | stdout                     |

All flags can also be set via environment variables (`WEB_LISTEN_ADDRESS`, `HBASE_MASTER_URL`, `HBASE_REGIONSERVER_URL`, `HBASE_IS_MASTER`, `LOG_LEVEL`, `LOG_FMT`, `LOG_OUTPUT`).



#### Master

Start in master:

```
./hbase_exporter --web.listen-address=":9003" --hbase.master.uri="http://localhost:60010/jmx" --hbase.master --hbase.instance="10.38.82.195:7005"
```

#### Regionserver

Start in regionserver:

```
./hbase_exporter --web.listen-address=":9003" --hbase.regionserver.uri="http://localhost:60030/jmx" --hbase.instance="10.38.82.195:7004"
```



### Metrics

#### common

> Common jvm metrics, both hmaster and regionservers.

> From(both hmaster and regionservers):
>
>  http://localhost:60030/jmx?qry=Hadoop:service=HBase,name=JvmMetrics and http://localhost:60010/jmx?qry=Hadoop:service=HBase,name=JvmMetrics
>
> Example:
>
> >  hbase_jvm_mem_non_heap_used_m{host="localhost",role="master"} 1
> >
> > hbase_jvm_mem_non_heap_used_m{host="localhost",role="regionserver"} 1

| Name                          | Type  | Origin in jmx   |
| ----------------------------- | ----- | --------------- |
| hbase_jvm_mem_non_heap_used_m | gauge | MemNonHeapUsedM |
| hbase_jvm_mem_heap_used_m     | gauge | MemHeapUsedM    |
| hbase_jvm_mem_heap_max_m      | gauge | MemHeapMaxM     |
| hbase_jvm_mem_max_m           | gauge | MemMaxM         |
| hbase_jvm_gc_time_millis      | gauge | GcTimeMillis    |
| hbase_jvm_gc_count            | gauge | GcCount         |
| hbase_jvm_thread_blocked      | gauge | ThreadsBlocked  |



#### JMX Java Agent compatibility

The Server, Region and Table collectors use the same public metric names and
labels as the HBase JMX Java agent configuration. Set `--hbase.instance` to
the legacy Agent address (`<node-ip>:7005` for HMaster, `<node-ip>:7004` for
RegionServer) when the default derived value is not appropriate.

- Master Server: `hbase_master_server_*{instance}`
- RegionServer Server: `hbase_regionserver_server_*{instance}`
- RegionServer Regions: `hbase_regionserver_region_*{instance,namespace,table,regionid}`
- RegionServer Tables: `hbase_regionserver_table_*{instance,namespace,table}`

#### Blackbox-style proxy mode

Use `--probe.enabled` to run one exporter as a proxy for multiple HBase
nodes. Each `/metrics` request scrapes exactly one target; metrics from separate
targets are never aggregated in one response.

```bash
./hbase_exporter --web.listen-address=":9102" --probe.enabled
```

The `target` parameter accepts `ip:port` or a full HTTP(S) JMX URL. The
standard HBase ports infer the role automatically (`16010` for HMaster and
`16030` for RegionServer); pass `role` explicitly for other ports.

```text
/metrics?target=172.28.24.101:16010&role=master
/metrics?target=172.28.24.104:16030&role=regionserver
/metrics?target=172.28.24.104:16030&role=regionserver&instance=172.28.24.104:7004
```

`instance` is optional. When omitted, the exporter derives `<host>:7005` for
HMaster and `<host>:7004` for RegionServer to match the Java agent labels.

#### HMaster

> HMaster server metrics, only for hmaster.
>
> From: http://localhost:60010/jmx?qry=Hadoop:service=HBase,name=Master,sub=Server
>
> Example: hbase_master_server_num_region_servers{instance="localhost:7005"}

| Name                                 | Type  | Origin in jmx        |
| ------------------------------------ | ----- | -------------------- |
| hbase_master_server_num_region_servers      | gauge | numRegionServers     |
| hbase_master_server_num_dead_region_servers | gauge | numDeadRegionServers |
| hbase_master_server_average_load            | gauge | averageLoad          |



#### Regionserver

>Regionserver server metrics, only for regionserver.
>
>From: http://localhost:60030/jmx?qry=Hadoop:service=HBase,name=RegionServer,sub=Server
>
>Example: hbase_regionserver_server_mem_store_size{instance="localhost:7004"} 1

| Name                                 | Type  | Origin in jmx         |
| ------------------------------------ | ----- | --------------------- |
| hbase_regionserver_server_mem_store_size          | gauge | memStoreSize          |
| hbase_regionserver_server_region_count            | gauge | regionCount           |
| hbase_regionserver_server_store_count             | gauge | storeCount            |
| hbase_regionserver_server_store_file_count        | gauge | storeFileCount        |
| hbase_regionserver_server_store_file_size         | gauge | storeFileSize         |
| hbase_regionserver_server_total_request_count     | gauge | totalRequestCount     |
| hbase_regionserver_server_read_request_count      | gauge | readRequestCount      |
| hbase_regionserver_server_write_request_count     | gauge | writeRequestCount     |
| hbase_regionserver_server_split_queue_length      | gauge | splitQueueLength      |
| hbase_regionserver_server_compaction_queue_length | gauge | compactionQueueLength |
| hbase_regionserver_server_flush_queue_length      | gauge | flushQueueLength      |
| hbase_regionserver_server_block_cache_count       | gauge | blockCacheCount       |
| hbase_regionserver_server_block_cache_size        | gauge | blockCacheSize        |
| hbase_regionserver_server_block_cache_hit_count   | gauge | blockCacheHitCount    |
| hbase_regionserver_server_block_cache_miss_count  | gauge | blockCacheMissCount   |
| hbase_regionserver_server_block_cache_count_hit_percent | gauge | blockCacheCountHitPercent |
| hbase_regionserver_server_slow_append_count       | gauge | slowAppendCount       |
| hbase_regionserver_server_slow_delete_count       | gauge | slowDeleteCount       |
| hbase_regionserver_server_slow_get_count          | gauge | slowGetCount          |
| hbase_regionserver_server_slow_put_count          | gauge | slowPutCount          |
| hbase_regionserver_server_slow_increment_count    | gauge | slowIncrementCount    |



> Regionserver region metrics, only for regionserver.
>
> From: http://localhost:60030/jmx?qry=Hadoop:service=HBase,name=RegionServer,sub=Regions
>
> Example:  hbase_regionserver_region_store_count{instance="localhost:7004",namespace="n1",table="t1",regionid="4fcaf7b9d1fedc1b62c15cbb1c9a10dc"} 1

| Name                                     | Type  | Origin in jmx             |
| ---------------------------------------- | ----- | ------------------------- |
| hbase_regionserver_region_store_count                 | gauge | storeCount                |
| hbase_regionserver_region_store_file_count            | gauge | storeFileCount            |
| hbase_regionserver_region_mem_store_size              | gauge | memStoreSize              |
| hbase_regionserver_region_store_file_size             | gauge | storeFileSize             |
| hbase_regionserver_region_compactions_completed_count | gauge | compactionsCompletedCount |
| hbase_regionserver_region_compactions_failed_count    | gauge | compactionsFailedCount    |
| hbase_regionserver_region_read_request_count          | gauge | readRequestCount          |
| hbase_regionserver_region_write_request_count         | gauge | writeRequestCount         |
| hbase_regionserver_region_total_request_count         | gauge | totalRequestCount         |
| hbase_regionserver_region_num_files_compacted_count   | gauge | numFilesCompactedCount    |
| hbase_regionserver_region_num_bytes_compacted_count   | gauge | numBytesCompactedCount    |
