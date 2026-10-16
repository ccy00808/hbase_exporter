package main

import (
	"net/url"
	"testing"
)

func TestParseProbeTarget(t *testing.T) {
	target, err := parseProbeTarget(url.Values{
		"target": []string{"172.28.24.104:16030"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if target.isMaster {
		t.Fatal("expected regionserver target")
	}
	if got, want := target.jmxURL.String(), "http://172.28.24.104:16030/jmx"; got != want {
		t.Fatalf("target URL = %q, want %q", got, want)
	}
	if got, want := target.instance, "172.28.24.104:7004"; got != want {
		t.Fatalf("instance = %q, want %q", got, want)
	}
}

func TestParseProbeTargetSupportsMasterAndInstanceOverride(t *testing.T) {
	target, err := parseProbeTarget(url.Values{
		"target":   []string{"https://hmaster.example:16010/jmx?ignored=true"},
		"role":     []string{"master"},
		"instance": []string{"10.38.82.195:7005"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !target.isMaster {
		t.Fatal("expected master target")
	}
	if got, want := target.jmxURL.String(), "https://hmaster.example:16010/jmx"; got != want {
		t.Fatalf("target URL = %q, want %q", got, want)
	}
	if got, want := target.instance, "10.38.82.195:7005"; got != want {
		t.Fatalf("instance = %q, want %q", got, want)
	}
}

func TestParseProbeTargetRejectsUnknownPortWithoutRole(t *testing.T) {
	_, err := parseProbeTarget(url.Values{"target": []string{"172.28.24.104:18030"}})
	if err == nil {
		t.Fatal("expected target without role to fail")
	}
}
