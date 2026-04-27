package workflows

import (
	"testing"
)

func TestParsePromResponse(t *testing.T) {
	raw := []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"node_cpu_seconds_total","instance":"10.0.1.5:9100","mode":"idle"},"value":[1713000000,"42.5"]},{"metric":{"__name__":"node_cpu_seconds_total","instance":"10.0.1.6:9100","mode":"idle"},"value":[1713000000,"38.2"]}]}}`)
	results, err := parsePromResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].MetricName != "node_cpu_seconds_total" {
		t.Errorf("name = %q, want node_cpu_seconds_total", results[0].MetricName)
	}
	if results[0].IP != "10.0.1.5" {
		t.Errorf("ip = %q, want 10.0.1.5", results[0].IP)
	}
	if results[0].Value != 42.5 {
		t.Errorf("value = %f, want 42.5", results[0].Value)
	}
}

func TestParsePromResponse_NoInstance(t *testing.T) {
	raw := []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"up"},"value":[1713000000,"1"]}]}}`)
	results, err := parsePromResponse(raw)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].IP != "" {
		t.Errorf("ip should be empty, got %q", results[0].IP)
	}
}

func TestParsePromResponse_Error(t *testing.T) {
	raw := []byte(`{"status":"error","errorType":"bad_data","error":"invalid query"}`)
	_, err := parsePromResponse(raw)
	if err == nil {
		t.Fatal("expected error")
	}
}

// TestParsePromResponse_MalformedJSON: a payload that isn't JSON at all
// must surface an unmarshal error, not a panic. Prometheus servers can
// return HTML error pages from upstream proxies during outages.
func TestParsePromResponse_MalformedJSON(t *testing.T) {
	t.Parallel()
	if _, err := parsePromResponse([]byte("<html>502 bad gateway</html>")); err == nil {
		t.Fatal("expected unmarshal error on non-JSON body")
	}
}

// TestParsePromResponse_SkipsBadValueRows: each malformed row should be
// dropped (continue), not abort the parse. Mixed payloads with one
// good + one bad row must return only the good one.
func TestParsePromResponse_SkipsBadValueRows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     []byte
		wantLen int
	}{
		{
			// Timestamp not a number → drop row.
			name:    "non-numeric timestamp",
			raw:     []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"good","instance":"a:1"},"value":[1,"2"]},{"metric":{"__name__":"bad","instance":"b:1"},"value":["nope","2"]}]}}`),
			wantLen: 1,
		},
		{
			// Value not a string → drop row (Prom values are stringy floats).
			name:    "non-string value",
			raw:     []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"good","instance":"a:1"},"value":[1,"2"]},{"metric":{"__name__":"bad","instance":"b:1"},"value":[1,2.5]}]}}`),
			wantLen: 1,
		},
		{
			// Value is a string but not parseable as float → drop row.
			name:    "unparseable float value",
			raw:     []byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{"__name__":"good","instance":"a:1"},"value":[1,"2"]},{"metric":{"__name__":"bad","instance":"b:1"},"value":[1,"NaNny"]}]}}`),
			wantLen: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePromResponse(tc.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantLen {
				t.Fatalf("expected %d rows after dropping malformed, got %d (%+v)", tc.wantLen, len(got), got)
			}
			if len(got) == 1 && got[0].MetricName != "good" {
				t.Errorf("kept the wrong row: got %q, want good", got[0].MetricName)
			}
		})
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct{ in, want string }{
		{"10.0.1.5:9100", "10.0.1.5"},
		{"10.0.1.5", "10.0.1.5"},
		{"hostname:9100", "hostname"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := extractIP(tt.in); got != tt.want {
			t.Errorf("extractIP(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
