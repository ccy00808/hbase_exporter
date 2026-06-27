package utils

import "testing"

func TestSplitHBaseRegionStr(t *testing.T) {
	tests := []struct {
		in   string
		want []string
		err  bool
	}{
		{"Namespace_n1_table_t1_region_r1_metric_storeCount", []string{"n1", "t1", "r1", "storeCount"}, false},
		{"Namespace_default_table_mytable_region_abc123_metric_readRequestCount", []string{"default", "mytable", "abc123", "readRequestCount"}, false},
		{"not_a_region_key", nil, true},
		{"Namespace_n1_table_t1", nil, true}, // missing region/metric
	}
	for _, tt := range tests {
		got, err := SplitHBaseRegionStr(tt.in)
		if tt.err {
			if err == nil {
				t.Errorf("%q: expected error, got %v", tt.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tt.in, err)
			continue
		}
		if len(got) != len(tt.want) {
			t.Errorf("%q: got %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("%q: got %v, want %v", tt.in, got, tt.want)
			}
		}
	}
}
