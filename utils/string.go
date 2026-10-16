package utils

import (
	"errors"
	"strings"
)

// SplitHBaseRegionStr splits a JMX key of the form
//
//	Namespace_n1_table_t1_region_r1_metric_m1
//
// into [namespace, table, region, metric] = [n1, t1, r1, m1].
//
// HBase region JMX keys are structured as
// "Namespace_<ns>_table_<t>_region_<r>_metric_<m>"; the legacy separators
// ("_region_", "_metric_") also match the metric/region segments, so we split
// off the four tagged segments from left to right.
func SplitHBaseRegionStr(data string) ([]string, error) {
	const (
		nsSep     = "Namespace_"
		tableSep  = "_table_"
		regionSep = "_region_"
		metricSep = "_metric_"
	)

	s, ok := cutPrefix(data, nsSep)
	if !ok {
		return nil, errors.New("missing Namespace_ prefix")
	}

	ns, s, ok := cutSegment(s, tableSep)
	if !ok {
		return nil, errors.New("missing _table_ separator")
	}
	table, s, ok := cutSegment(s, regionSep)
	if !ok {
		return nil, errors.New("missing _region_ separator")
	}
	region, metric, ok := cutSegment(s, metricSep)
	if !ok {
		return nil, errors.New("missing _metric_ separator")
	}

	return []string{ns, table, region, metric}, nil
}

// SplitHBaseTableStr splits a JMX key of the form
//
//	Namespace_<ns>_table_<table>_metric_<metric>
//
// into [namespace, table, metric]. Table and region values are exposed by
// different RegionServer MBeans; callers should route region keys separately.
func SplitHBaseTableStr(data string) ([]string, error) {
	const (
		nsSep     = "Namespace_"
		tableSep  = "_table_"
		metricSep = "_metric_"
	)

	s, ok := cutPrefix(data, nsSep)
	if !ok {
		return nil, errors.New("missing Namespace_ prefix")
	}
	ns, s, ok := cutSegment(s, tableSep)
	if !ok {
		return nil, errors.New("missing _table_ separator")
	}
	table, metric, ok := cutSegment(s, metricSep)
	if !ok {
		return nil, errors.New("missing _metric_ separator")
	}
	return []string{ns, table, metric}, nil
}

// cutPrefix reports whether s starts with prefix and returns the remainder.
func cutPrefix(s, prefix string) (string, bool) {
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	return s[len(prefix):], true
}

// cutSegment splits s at the first occurrence of sep into (before, after).
func cutSegment(s, sep string) (before, after string, ok bool) {
	if i := strings.Index(s, sep); i >= 0 {
		return s[:i], s[i+len(sep):], true
	}
	return s, "", false
}
