package facebookmessenger

import "testing"

func TestMetrics_IncrementsAndSnapshot(t *testing.T) {
	ResetMetrics()

	IncInbound()
	IncInbound()
	IncOutbound()
	IncRateLimited()
	IncSignatureFail()
	IncReconnect()
	IncCheckpoint()

	m := Metrics()
	want := map[string]int64{
		"fbm_inbound_total":        2,
		"fbm_outbound_total":       1,
		"fbm_rate_limited_total":   1,
		"fbm_signature_fail_total": 1,
		"fbm_reconnect_total":      1,
		"fbm_checkpoint_total":     1,
	}
	for k, v := range want {
		if got := m[k]; got != v {
			t.Errorf("%s: got=%d want=%d", k, got, v)
		}
	}
}

func TestMetrics_Reset(t *testing.T) {
	IncInbound()
	IncOutbound()
	ResetMetrics()
	m := Metrics()
	for k, v := range m {
		if v != 0 {
			t.Errorf("%s not reset: %d", k, v)
		}
	}
}
