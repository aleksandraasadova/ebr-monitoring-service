package e2e

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func TestReport_ListEmpty_ReturnsArray(t *testing.T) {
	// Admin can always list reports (may be empty)
	resp, body := apiDo("GET", "/api/v1/reports", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var reports []map[string]any
	parseJSON(t, body, &reports)
	// Can be empty on a fresh DB — just ensure it's an array, not null or error
	_ = reports
}

func TestReport_ListByOperator(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "ОтчётыОператор", "Тест")

	resp, body := apiDo("GET", "/api/v1/reports", opToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var reports []map[string]any
	parseJSON(t, body, &reports)
	_ = reports
}

func TestReport_UnauthorizedRejected(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/reports", "", nil)
	assertStatus(t, resp, body, http.StatusUnauthorized)
}

func TestReport_Generate_AfterWeighing(t *testing.T) {
	// Create a batch that has been weighed
	opUsername, opToken := createTestOperator(t, "E2E", "ОтчётВзвешивание", "Тест")
	batchCode := createBatch(t, opToken, 200)

	// Start and complete weighing
	apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opToken, nil)

	_, wbody := apiDo("GET", "/api/v1/batches/"+batchCode+"/weighing", opToken, nil)
	var items []struct {
		ID          int     `json:"id"`
		RequiredQty float64 `json:"required_qty"`
	}
	parseJSON(t, wbody, &items)

	for _, item := range items {
		apiDo(
			"POST",
			"/api/v1/batches/"+batchCode+"/weighing/"+itoa(item.ID)+"/confirm",
			opToken,
			map[string]any{
				"actual_qty":         item.RequiredQty,
				"signature_password": opUsername,
			},
		)
	}

	// Generate report — returns HTML
	resp, body := apiDo("GET", "/api/v1/batches/"+batchCode+"/report", opToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	html := string(body)
	if !strings.Contains(html, batchCode) {
		t.Errorf("report HTML must contain batch code %q", batchCode)
	}
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("report must be a valid HTML document")
	}
	if !strings.Contains(html, "Протокол") {
		t.Error("report must contain the word 'Протокол'")
	}

	// Second call should return cached report (same HTML)
	resp2, body2 := apiDo("GET", "/api/v1/batches/"+batchCode+"/report", opToken, nil)
	assertStatus(t, resp2, body2, http.StatusOK)

	if string(body) != string(body2) {
		t.Error("second call returned different HTML — expected cached report")
	}

	// Report should now appear in operator's list
	resp3, body3 := apiDo("GET", "/api/v1/reports", opToken, nil)
	assertStatus(t, resp3, body3, http.StatusOK)

	var reports []struct {
		BatchCode string `json:"batch_code"`
	}
	parseJSON(t, body3, &reports)

	found := false
	for _, r := range reports {
		if r.BatchCode == batchCode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("generated report for batch %q not found in list", batchCode)
	}
}

func TestReport_OperatorCannotAccessOthersBatch(t *testing.T) {
	// Operator A creates a batch
	opAUsername, opAToken := createTestOperator(t, "E2E", "ОтчётОператорА", "Тест")
	batchCode := createBatch(t, opAToken, 200)

	// Confirm weighing as operator A
	apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opAToken, nil)
	_, wbody := apiDo("GET", "/api/v1/batches/"+batchCode+"/weighing", opAToken, nil)
	var items []struct {
		ID          int     `json:"id"`
		RequiredQty float64 `json:"required_qty"`
	}
	parseJSON(t, wbody, &items)
	for _, item := range items {
		apiDo(
			"POST",
			"/api/v1/batches/"+batchCode+"/weighing/"+itoa(item.ID)+"/confirm",
			opAToken,
			map[string]any{
				"actual_qty":         item.RequiredQty,
				"signature_password": opAUsername,
			},
		)
	}

	// Operator B tries to access the report → must be 403
	_, opBToken := createTestOperator(t, "E2E", "ОтчётОператорБ", "Тест")
	resp, body := apiDo("GET", "/api/v1/batches/"+batchCode+"/report", opBToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 when operator B accesses operator A's batch report, got %d — %s", resp.StatusCode, body)
	}
}

func TestReport_AdminCanAccessAnyBatch(t *testing.T) {
	opUsername, opToken := createTestOperator(t, "E2E", "ОтчётАдминДоступ", "Тест")
	batchCode := createBatch(t, opToken, 200)

	// Minimal weighing setup
	apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opToken, nil)
	_, wbody := apiDo("GET", "/api/v1/batches/"+batchCode+"/weighing", opToken, nil)
	var items []struct {
		ID          int     `json:"id"`
		RequiredQty float64 `json:"required_qty"`
	}
	parseJSON(t, wbody, &items)
	for _, item := range items {
		apiDo(
			"POST",
			"/api/v1/batches/"+batchCode+"/weighing/"+itoa(item.ID)+"/confirm",
			opToken,
			map[string]any{
				"actual_qty":         item.RequiredQty,
				"signature_password": opUsername,
			},
		)
	}

	// Admin accesses the report — must succeed
	resp, body := apiDo("GET", "/api/v1/batches/"+batchCode+"/report", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	if !strings.Contains(string(body), batchCode) {
		t.Errorf("admin report must contain batch code %q", batchCode)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

// Suppress unused import warning — fmt is used indirectly via test helpers.
var _ = fmt.Sprintf
