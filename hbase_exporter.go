package main

import (
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"

	"github.com/daiwei233/hbase_exporter/collector"
)

const name = "hbase_exporter"

func getLogger(loglevel, logoutput, logfmt string) log.Logger {
	out := os.Stdout
	if strings.EqualFold(logoutput, "stderr") {
		out = os.Stderr
	}

	var newLogger func(io.Writer) log.Logger
	switch strings.ToLower(logfmt) {
	case "json":
		newLogger = log.NewJSONLogger
	default:
		newLogger = log.NewLogfmtLogger
	}

	logger := newLogger(log.NewSyncWriter(out))

	var opt level.Option
	switch strings.ToLower(loglevel) {
	case "debug":
		opt = level.AllowDebug()
	case "warn":
		opt = level.AllowWarn()
	case "error":
		opt = level.AllowError()
	default:
		opt = level.AllowInfo()
	}
	return log.With(level.NewFilter(logger, opt),
		"ts", log.DefaultTimestampUTC,
		"caller", log.DefaultCaller,
	)
}

func main() {
	listenAddress := kingpin.Flag("web.listen-address",
		"Address to listen on for web interface and telemetry.").
		Default(":9115").Envar("WEB_LISTEN_ADDRESS").String()
	metricsPath := kingpin.Flag("web.telemetry-path",
		"Path under which to expose metrics.").
		Default("/metrics").Envar("WEB_TELEMETRY_PATH").String()
	hbaseMasterURI := kingpin.Flag("hbase.master.uri",
		"HTTP jmx address of an HBase master node.").
		Default("http://localhost:60010/jmx").Envar("HBASE_MASTER_URL").String()
	hbaseRegionserverURI := kingpin.Flag("hbase.regionserver.uri",
		"HTTP jmx address of an HBase regionserver node.").
		Default("http://localhost:60030/jmx").Envar("HBASE_REGIONSERVER_URL").String()
	hbaseIsMaster := kingpin.Flag("hbase.master",
		"Scrape the HBase master (instead of a regionserver).").
		Default("false").Envar("HBASE_IS_MASTER").Bool()
	hbaseInstance := kingpin.Flag("hbase.instance",
		"Compatibility instance label; defaults to the JMX host with port 7005 for HMaster or 7004 for RegionServer.").
		Default("").Envar("HBASE_INSTANCE").String()
	probeEnabled := kingpin.Flag("probe.enabled",
		"Enable the Blackbox-style /metrics endpoint for scraping a target supplied per request.").
		Default("false").Envar("PROBE_ENABLED").Bool()
	logLevel := kingpin.Flag("log.level",
		"Sets the loglevel. Valid levels are debug, info, warn, error").
		Default("info").Envar("LOG_LEVEL").String()
	logFormat := kingpin.Flag("log.format",
		"Sets the log format. Valid formats are json and logfmt").
		Default("logfmt").Envar("LOG_FMT").String()
	logOutput := kingpin.Flag("log.output",
		"Sets the log output. Valid outputs are stdout and stderr").
		Default("stdout").Envar("LOG_OUTPUT").String()

	kingpin.Version(version.Print(name))
	kingpin.CommandLine.HelpFlag.Short('h')
	kingpin.Parse()

	logger := getLogger(*logLevel, *logOutput, *logFormat)

	masterURL, err := url.Parse(*hbaseMasterURI)
	if err != nil {
		_ = level.Error(logger).Log("msg", "failed to parse hbase.master.uri", "err", err)
		os.Exit(1)
	}
	regionserverURL, err := url.Parse(*hbaseRegionserverURI)
	if err != nil {
		_ = level.Error(logger).Log("msg", "failed to parse hbase.regionserver.uri", "err", err)
		os.Exit(1)
	}

	prometheus.MustRegister(newBuildInfoCollector(name))

	if !*probeEnabled {
		targetURL := regionserverURL
		isMaster := *hbaseIsMaster
		if isMaster {
			targetURL = masterURL
		}
		instance := compatibilityInstance(*hbaseInstance, targetURL, isMaster)
		if err := registerTargetCollectors(prometheus.DefaultRegisterer, logger, targetURL, isMaster, instance); err != nil {
			_ = level.Error(logger).Log("msg", "failed to register HBase collectors", "err", err)
			os.Exit(1)
		}
	}

	level.Info(logger).Log("msg", "Build context", "build_context", version.BuildContext())
	level.Info(logger).Log("msg", "Starting hbase_exporter", "version", version.Info())

	handler := newHTTPHandler(*metricsPath, *probeEnabled, logger)
	level.Info(logger).Log("msg", "Listening on", "address", *listenAddress)
	_ = http.ListenAndServe(*listenAddress, handler)
}

func registerTargetCollectors(registerer prometheus.Registerer, logger log.Logger, targetURL *url.URL, isMaster bool, instance string) error {
	collectors := []prometheus.Collector{collector.NewJvm(logger, targetURL)}
	if isMaster {
		collectors = append(collectors, collector.NewMasterServer(logger, targetURL, instance))
	} else {
		collectors = append(collectors,
			collector.NewRsServer(logger, targetURL, instance),
			collector.NewRsRegion(logger, targetURL, instance),
			collector.NewRsTable(logger, targetURL, instance),
		)
	}
	for _, c := range collectors {
		if err := registerer.Register(c); err != nil {
			return err
		}
	}
	return nil
}

func newHTTPHandler(metricsPath string, probeEnabled bool, logger log.Logger) http.Handler {
	mux := http.NewServeMux()
	if probeEnabled {
		mux.Handle(metricsPath, newProbeHandler(logger))
	} else {
		mux.Handle(metricsPath, promhttp.Handler())
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		body := `<html><head><title>HBase Exporter</title></head><body><h1>HBase Exporter</h1><p><a href='` + metricsPath + `'>Metrics</a></p>`
		if probeEnabled {
			body += `<p>Metrics: <code>` + metricsPath + `?target=&lt;ip:port&gt;&amp;role=master|regionserver</code></p>`
		}
		body += `</body></html>`
		_, _ = w.Write([]byte(body))
	})
	return mux
}

func compatibilityInstance(configured string, u *url.URL, isMaster bool) string {
	if configured != "" {
		return configured
	}
	port := "7004"
	if isMaster {
		port = "7005"
	}
	return net.JoinHostPort(u.Hostname(), port)
}
