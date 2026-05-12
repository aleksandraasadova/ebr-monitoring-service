package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// batchReadyForProcess creates a batch and confirms all weighing items so it reaches ready_for_process.
// Returns batchCode, opUsername, opToken.
func batchReadyForProcess(t *testing.T, surname string) (batchCode, opUsername, opToken string) {
	t.Helper()
	opUsername, opToken = createTestOperator(t, "E2E", surname, "Процесс")
	batchCode = createBatch(t, opToken, 200)

	// Start weighing
	resp, body := apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opToken, nil)
	assertStatus(t, resp, body, http.StatusNoContent)

	// Get weighing log
	_, wbody := apiDo("GET", "/api/v1/batches/"+batchCode+"/weighing", opToken, nil)
	var items []struct {
		ID          int     `json:"id"`
		RequiredQty float64 `json:"required_qty"`
	}
	parseJSON(t, wbody, &items)

	// Confirm all
	for _, item := range items {
		r, b := apiDo(
			"POST",
			fmt.Sprintf("/api/v1/batches/%s/weighing/%d/confirm", batchCode, item.ID),
			opToken,
			map[string]any{
				"actual_qty":         item.RequiredQty,
				"signature_password": opUsername,
			},
		)
		assertStatus(t, r, b, http.StatusOK)
	}
	return
}

// TestProcessStart_EquipmentOffline — in test env there's no PLC/MQTT so equipment is offline.
// The system must reject process start with 409.
func TestProcessStart_EquipmentOffline(t *testing.T) {
	batchCode, opUsername, opToken := batchReadyForProcess(t, "ОборудованиеОффлайн")

	resp, body := apiDo("POST", "/api/v1/batches/"+batchCode+"/process/start", opToken, map[string]string{
		"password": opUsername,
	})
	// Equipment is offline → must be 409
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 (equipment offline), got %d — %s", resp.StatusCode, body)
	}
}

func TestProcessStart_WrongPassword(t *testing.T) {
	batchCode, _, opToken := batchReadyForProcess(t, "НеверныйПарольПроцесс")

	resp, body := apiDo("POST", "/api/v1/batches/"+batchCode+"/process/start", opToken, map[string]string{
		"password": "absolutely_wrong",
	})
	// Wrong password → 403 (checked before equipment)
	assertStatus(t, resp, body, http.StatusForbidden)
}

func TestProcessStart_AdminForbidden(t *testing.T) {
	_, _, opToken := batchReadyForProcess(t, "АдминЗапускПроцесс")
	// Create a batch with operator, then try to start with admin → 403
	batchCode := createBatch(t, opToken, 200)
	apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opToken, nil)

	resp, body := apiDo("POST", "/api/v1/batches/"+batchCode+"/process/start", adminToken, map[string]string{
		"password": "admin01",
	})
	assertStatus(t, resp, body, http.StatusForbidden)
}

func TestGetStages_BeforeProcessStart(t *testing.T) {
	batchCode, _, opToken := batchReadyForProcess(t, "СтадииДоСтарта")

	// No stages exist yet — should return empty array, not error
	resp, body := apiDo("GET", "/api/v1/batches/"+batchCode+"/process/stages", opToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var stages []map[string]any
	parseJSON(t, body, &stages)

	if len(stages) != 0 {
		t.Errorf("expected 0 stages before process start, got %d", len(stages))
	}
}

func TestGetCurrentStage_BeforeStart_NotFound(t *testing.T) {
	batchCode, _, opToken := batchReadyForProcess(t, "ТекущаяСтадияПусто")

	resp, body := apiDo("GET", "/api/v1/batches/"+batchCode+"/process/current", opToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 before process start, got %d — %s", resp.StatusCode, body)
	}
}

func TestEvents_CreateAndList(t *testing.T) {
	batchCode, _, opToken := batchReadyForProcess(t, "СобытияСоздание")

	// Create an event
	resp, body := apiDo("POST", "/api/v1/batches/"+batchCode+"/events", opToken, map[string]string{
		"type":        "deviation",
		"severity":    "warning",
		"description": "E2E test event — температура выше нормы",
	})
	assertStatus(t, resp, body, http.StatusCreated)

	var created struct {
		ID          int    `json:"id"`
		Type        string `json:"type"`
		Severity    string `json:"severity"`
		Description string `json:"description"`
		StageKey    string `json:"stage_key"`
	}
	parseJSON(t, body, &created)

	if created.ID == 0 {
		t.Error("created event must have non-zero ID")
	}
	if created.Type != "deviation" {
		t.Errorf("expected type=deviation, got %q", created.Type)
	}
	if created.Severity != "warning" {
		t.Errorf("expected severity=warning, got %q", created.Severity)
	}

	// List events for batch
	resp2, body2 := apiDo("GET", "/api/v1/batches/"+batchCode+"/events", opToken, nil)
	assertStatus(t, resp2, body2, http.StatusOK)

	var events []struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	}
	parseJSON(t, body2, &events)

	if len(events) == 0 {
		t.Fatal("expected at least one event in list")
	}

	found := false
	for _, e := range events {
		if e.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("created event %d not found in list", created.ID)
	}
}

func TestEvents_Resolve(t *testing.T) {
	batchCode, _, opToken := batchReadyForProcess(t, "СобытияРезолв")

	// Create event
	resp, body := apiDo("POST", "/api/v1/batches/"+batchCode+"/events", opToken, map[string]string{
		"type":        "alarm",
		"severity":    "critical",
		"description": "E2E критическое отклонение",
	})
	assertStatus(t, resp, body, http.StatusCreated)

	var created struct {
		ID int `json:"id"`
	}
	parseJSON(t, body, &created)

	// Resolve the event with a comment
	resp2, body2 := apiDo(
		"POST",
		fmt.Sprintf("/api/v1/events/%d/resolve", created.ID),
		opToken,
		map[string]string{
			"comment": "Проверено и устранено в ходе E2E теста",
		},
	)
	assertStatus(t, resp2, body2, http.StatusNoContent)
}

func TestEvents_InvalidEventID(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/events/0/resolve", adminToken, map[string]string{
		"comment": "test",
	})
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 400 or 404 for invalid event id, got %d — %s", resp.StatusCode, body)
	}
}

func TestEvents_NonExistentEvent_NotFound(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/events/999999/resolve", adminToken, map[string]string{
		"comment": "ghost",
	})
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 404 for non-existent event, got %d — %s", resp.StatusCode, body)
	}
}
