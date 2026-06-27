package collector

import (
	"net/url"
	"strings"

	"github.com/go-kit/kit/log"
)

// MasterServer is the Prometheus collector for the HBase Master Server bean.
// Only registered on the master-scraping instance.
type MasterServer struct {
	*jmxCollector[masterServerResponse]
}

func NewMasterServer(logger log.Logger, u *url.URL) *MasterServer {
	labels := func(r masterServerResponse) []string { return hostRoleLabels(r.Host, r.Role) }
	metrics := []metricDef[masterServerResponse]{
		gauge(subsystemServer, "average_load", "Average number of regions per regionserver.", labels, func(r masterServerResponse) float64 { return r.AverageLoad }),
		gauge(subsystemServer, "num_region_servers", "Number of live regionservers.", labels, func(r masterServerResponse) float64 { return float64(r.NumRegionServers) }),
		gauge(subsystemServer, "num_dead_region_servers", "Number of dead regionservers.", labels, func(r masterServerResponse) float64 { return float64(r.NumDeadRegionServers) }),
		gauge(subsystemServer, "is_active_master", "1 if this is the active master, 0 otherwise.", labels, func(r masterServerResponse) float64 { return boolFloat(r.IsActiveMaster) }),
	}
	return &MasterServer{newJmxCollector(logger, u, subsystemServer, "Hadoop:service=HBase,name=Master,sub=Server", metrics)}
}

// boolFloat maps a JMX boolean string ("true"/"false") to 1.0/0.0.
func boolFloat(s string) float64 {
	if strings.EqualFold(s, "true") {
		return 1
	}
	return 0
}
