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
