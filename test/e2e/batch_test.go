package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// createBatch creates a batch as the given operator and returns the batch_code.
func createBatch(t *testing.T, opToken string, volumeL int) string {
	t.Helper()
	resp, body := apiDo("POST", "/api/v1/batches", opToken, map[string]any{
		"recipe_code":    testRecipeCode,
		"target_volume_l": volumeL,
	})
	assertStatus(t, resp, body, http.StatusCreated)

	var res struct {
		BatchCode string `json:"batch_code"`
	}
	parseJSON(t, body, &res)
	if res.BatchCode == "" {
		t.Fatal("created batch has empty batch_code")
	}
	return res.BatchCode
}

func TestBatch_Create_Success(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "СозданиеПартии", "Тест")
	batchCode := createBatch(t, opToken, 200)

	if batchCode == "" {
		t.Fatal("expected non-empty batch_code")
	}
}

func TestBatch_Create_InvalidVolume(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "НеверныйОбъем", "Тест")

	// Volume below min (100) → must fail
	resp, body := apiDo("POST", "/api/v1/batches", opToken, map[string]any{
		"recipe_code":    testRecipeCode,
		"target_volume_l": 10,
	})
	if resp.StatusCode == http.StatusCreated {
		t.Errorf("expected failure for volume below min, got 201 — %s", body)
	}
}

func TestBatch_Create_InvalidRecipe(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "НеверныйРецепт", "Тест")

	resp, body := apiDo("POST", "/api/v1/batches", opToken, map[string]any{
		"recipe_code":    "RC-DOES-NOT-EXIST",
		"target_volume_l": 200,
	})
	if resp.StatusCode == http.StatusCreated {
		t.Errorf("expected failure for nonexistent recipe, got 201 — %s", body)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for nonexistent recipe, got %d — %s", resp.StatusCode, body)
	}
}

func TestBatch_Create_AdminForbidden(t *testing.T) {
	// Only operators can create batches
	resp, body := apiDo("POST", "/api/v1/batches", adminToken, map[string]any{
		"recipe_code":    testRecipeCode,
		"target_volume_l": 200,
	})
	assertStatus(t, resp, body, http.StatusForbidden)
}

func TestBatch_ListByStatus(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "СписокПартий", "Тест")
	batchCode := createBatch(t, opToken, 200)

	// Immediately after creation the batch is in waiting_weighing status
	resp, body := apiDo("GET", "/api/v1/batches?status=waiting_weighing", opToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var batches []struct {
		BatchCode   string `json:"batch_code"`
		BatchStatus string `json:"batch_status"`
		RecipeCode  string `json:"recipe_code"`
	}
	parseJSON(t, body, &batches)

	found := false
	for _, b := range batches {
		if b.BatchCode == batchCode {
			found = true
			if b.BatchStatus != "waiting_weighing" {
				t.Errorf("expected status=waiting_weighing, got %q", b.BatchStatus)
			}
			if b.RecipeCode != testRecipeCode {
				t.Errorf("expected recipe_code=%q, got %q", testRecipeCode, b.RecipeCode)
			}
			break
		}
	}
	if !found {
		t.Errorf("batch %q not found in waiting_weighing list", batchCode)
	}
}

func TestBatch_ListByStatus_MissingQueryParam(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/batches", adminToken, nil)
	assertStatus(t, resp, body, http.StatusBadRequest)
}

func TestBatch_WeighingLifecycle(t *testing.T) {
	username, opToken := createTestOperator(t, "E2E", "Взвешивание", "Полное")
	batchCode := createBatch(t, opToken, 200)

	// 1. Start weighing
	resp, body := apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opToken, nil)
	assertStatus(t, resp, body, http.StatusNoContent)

	// 2. Batch should now be in weighing_in_progress
	resp2, body2 := apiDo("GET", "/api/v1/batches?status=weighing_in_progress", opToken, nil)
	assertStatus(t, resp2, body2, http.StatusOK)

	var batches []struct {
		BatchCode string `json:"batch_code"`
	}
	parseJSON(t, body2, &batches)

	found := false
	for _, b := range batches {
		if b.BatchCode == batchCode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("batch %q not found in weighing_in_progress after start", batchCode)
	}

	// 3. Get weighing log
	resp3, body3 := apiDo("GET", "/api/v1/batches/"+batchCode+"/weighing", opToken, nil)
	assertStatus(t, resp3, body3, http.StatusOK)

	var items []struct {
		ID          int     `json:"id"`
		Ingredient  string  `json:"ingredient"`
		RequiredQty float64 `json:"required_qty"`
		ActualQty   *float64 `json:"actual_qty"`
	}
	parseJSON(t, body3, &items)

	if len(items) == 0 {
		t.Fatal("weighing log must have at least one item for a recipe with ingredients")
	}

	// 4. Confirm all weighing items with e-signature (password = username)
	for _, item := range items {
		// Required qty is the actual value from the recipe calculation
		actualQty := item.RequiredQty // confirm exactly the required amount

		resp4, body4 := apiDo(
			"POST",
			fmt.Sprintf("/api/v1/batches/%s/weighing/%d/confirm", batchCode, item.ID),
			opToken,
			map[string]any{
				"actual_qty":         actualQty,
				"signature_password": username, // password = username (set by UserService)
			},
		)
		assertStatus(t, resp4, body4, http.StatusOK)

		var confirmRes struct {
			BatchStatus string `json:"batch_status"`
		}
		parseJSON(t, body4, &confirmRes)

		// Status should be either weighing_in_progress or ready_for_process
		if confirmRes.BatchStatus != "weighing_in_progress" && confirmRes.BatchStatus != "ready_for_process" {
			t.Errorf("unexpected status after confirm: %q", confirmRes.BatchStatus)
		}
	}

	// 5. After confirming all items, status must be ready_for_process
	resp5, body5 := apiDo("GET", "/api/v1/batches?status=ready_for_process", opToken, nil)
	assertStatus(t, resp5, body5, http.StatusOK)

	var rfpBatches []struct {
		BatchCode string `json:"batch_code"`
	}
	parseJSON(t, body5, &rfpBatches)

	found = false
	for _, b := range rfpBatches {
		if b.BatchCode == batchCode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("batch %q not found in ready_for_process after confirming all items", batchCode)
	}
}

func TestBatch_ConfirmWeighing_WrongPassword(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "НеверныйПароль", "Взвешивание")
	batchCode := createBatch(t, opToken, 200)

	// Start weighing
	apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opToken, nil)

	// Get item IDs
	_, body := apiDo("GET", "/api/v1/batches/"+batchCode+"/weighing", opToken, nil)
	var items []struct {
		ID int `json:"id"`
	}
	parseJSON(t, body, &items)

	if len(items) == 0 {
		t.Skip("no weighing items to test signature with")
	}

	// Try to confirm with wrong password → 403
	resp, body2 := apiDo(
		"POST",
		fmt.Sprintf("/api/v1/batches/%s/weighing/%d/confirm", batchCode, items[0].ID),
		opToken,
		map[string]any{
			"actual_qty":         10.0,
			"signature_password": "wrong_password_definitely",
		},
	)
	assertStatus(t, resp, body2, http.StatusForbidden)
}

func TestBatch_StartWeighing_Twice_Fails(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "ДвойноеВзвешивание", "Тест")
	batchCode := createBatch(t, opToken, 200)

	// Start once — OK
	resp1, body1 := apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opToken, nil)
	assertStatus(t, resp1, body1, http.StatusNoContent)

	// Start again — must fail (wrong status)
	resp2, body2 := apiDo("POST", "/api/v1/batches/"+batchCode+"/weighing/start", opToken, nil)
	if resp2.StatusCode != http.StatusConflict && resp2.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 409 on second start, got %d — %s", resp2.StatusCode, body2)
	}
}
