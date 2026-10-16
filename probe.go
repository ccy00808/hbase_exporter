package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// probeTarget is the request-scoped configuration for one HBase node. A probe
// never aggregates nodes: each request owns a fresh registry and a single JMX
// URL, which keeps concurrent Falcon scrapes isolated.
type probeTarget struct {
	jmxURL   *url.URL
	isMaster bool
	instance string
}

func newProbeHandler(logger log.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", http.MethodGet)
			http.Error(w, "only GET is supported", http.StatusMethodNotAllowed)
			return
		}

		target, err := parseProbeTarget(r.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		registry := prometheus.NewRegistry()
		if err := registerTargetCollectors(registry, logger, target.jmxURL, target.isMaster, target.instance); err != nil {
			_ = level.Error(logger).Log("msg", "failed to register probe collectors", "err", err)
			http.Error(w, "failed to prepare target collectors", http.StatusInternalServerError)
			return
		}

		promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(w, r)
	})
}

func parseProbeTarget(values url.Values) (probeTarget, error) {
	rawTarget := strings.TrimSpace(values.Get("target"))
	if rawTarget == "" {
		return probeTarget{}, fmt.Errorf("missing required target parameter")
	}
	if !strings.Contains(rawTarget, "://") {
		rawTarget = "http://" + rawTarget
	}

	targetURL, err := url.Parse(rawTarget)
	if err != nil {
		return probeTarget{}, fmt.Errorf("invalid target: %w", err)
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		return probeTarget{}, fmt.Errorf("target scheme must be http or https")
	}
	if targetURL.User != nil || targetURL.Hostname() == "" || targetURL.Port() == "" {
		return probeTarget{}, fmt.Errorf("target must be an IP or host with port")
	}
	switch targetURL.Path {
	case "", "/":
		targetURL.Path = "/jmx"
	case "/jmx":
	default:
		return probeTarget{}, fmt.Errorf("target path must be /jmx")
	}
	targetURL.RawQuery = ""
	targetURL.Fragment = ""

	isMaster, err := probeRole(values.Get("role"), targetURL.Port())
	if err != nil {
		return probeTarget{}, err
	}
	instance := strings.TrimSpace(values.Get("instance"))
	if strings.ContainsAny(instance, "\r\n") {
		return probeTarget{}, fmt.Errorf("instance must not contain a newline")
	}
	instance = compatibilityInstance(instance, targetURL, isMaster)

	return probeTarget{jmxURL: targetURL, isMaster: isMaster, instance: instance}, nil
}

func probeRole(value, port string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "master":
		return true, nil
	case "regionserver":
		return false, nil
	case "":
		switch port {
		case "16010":
			return true, nil
		case "16030":
			return false, nil
		default:
			return false, fmt.Errorf("missing role parameter; use master or regionserver")
		}
	default:
		return false, fmt.Errorf("invalid role %q; use master or regionserver", value)
	}
}
