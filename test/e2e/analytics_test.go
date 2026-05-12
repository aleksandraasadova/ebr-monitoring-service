package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestAnalytics_AdminGetsAll(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/analytics?days=30", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var result map[string]json.RawMessage
	parseJSON(t, body, &result)

	requiredFields := []string{
		"summary",
		"batch_by_day",
		"cycle_times",
		"status_breakdown",
		"events_by_stage",
		"events_per_batch",
		"avg_homog_temp",
	}
	for _, field := range requiredFields {
		if _, ok := result[field]; !ok {
			t.Errorf("analytics response missing field %q", field)
		}
	}
}

func TestAnalytics_DefaultDays(t *testing.T) {
	// No ?days param → defaults to 30
	resp, body := apiDo("GET", "/api/v1/analytics", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var result map[string]json.RawMessage
	parseJSON(t, body, &result)

	if _, ok := result["summary"]; !ok {
		t.Error("analytics response missing 'summary'")
	}
}

func TestAnalytics_OperatorSeesOwnData(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "АналитикаОператор", "Тест")

	resp, body := apiDo("GET", "/api/v1/analytics?days=30", opToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var result map[string]json.RawMessage
	parseJSON(t, body, &result)

	if _, ok := result["summary"]; !ok {
		t.Error("operator analytics response missing 'summary'")
	}
}

func TestAnalytics_SummaryStructure(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/analytics?days=365", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var result struct {
		Summary struct {
			TotalBatches    *int     `json:"total_batches"`
			CompletedBatches *int    `json:"completed_batches"`
			AvgCycleHours  *float64 `json:"avg_cycle_hours"`
		} `json:"summary"`
	}
	parseJSON(t, body, &result)

	// Summary must be present (non-nil pointer means the field exists in JSON)
	// Values can be 0 in a fresh DB, just verify structure is correct
	if result.Summary.TotalBatches == nil {
		t.Error("summary.total_batches is missing")
	}
}

func TestAnalytics_UnauthorizedRejected(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/analytics", "", nil)
	assertStatus(t, resp, body, http.StatusUnauthorized)
}

func TestAnalytics_DaysClampedAt365(t *testing.T) {
	// days=999 → clamped to 365, still returns 200
	resp, body := apiDo("GET", "/api/v1/analytics?days=999", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)
}
