package e2e

import (
	"net/http"
	"testing"
)

func TestRecipe_GetByCode_Exists(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/recipes/"+testRecipeCode, adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var res struct {
		Name       string `json:"name"`
		MinVolumeL int    `json:"min_volume_l"`
		MaxVolumeL int    `json:"max_volume_l"`
	}
	parseJSON(t, body, &res)

	if res.Name == "" {
		t.Error("recipe name must not be empty")
	}
	if res.MinVolumeL <= 0 || res.MaxVolumeL <= 0 {
		t.Errorf("expected positive volumes, got min=%d max=%d", res.MinVolumeL, res.MaxVolumeL)
	}
}

func TestRecipe_GetByCode_NotFound(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/recipes/RC-DOES-NOT-EXIST-9999", adminToken, nil)
	assertStatus(t, resp, body, http.StatusNotFound)
}

func TestRecipe_GetAll_AdminSeesAll(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/recipes", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var recipes []map[string]any
	parseJSON(t, body, &recipes)

	// At least the test recipe must be present
	found := false
	for _, r := range recipes {
		if r["recipe_code"] == testRecipeCode {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("test recipe %q not found in list", testRecipeCode)
	}
}

func TestRecipe_GetAll_OperatorForbidden(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "РецептЗапрет", "Тестовый")
	resp, body := apiDo("GET", "/api/v1/recipes", opToken, nil)
	assertStatus(t, resp, body, http.StatusForbidden)
}

func TestRecipe_OperatorCanGetByCode(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "РецептЧтение", "Тестовый")
	resp, body := apiDo("GET", "/api/v1/recipes/"+testRecipeCode, opToken, nil)
	assertStatus(t, resp, body, http.StatusOK)
}

func TestRecipe_CreateAndArchive(t *testing.T) {
	// Get ingredients
	resp, body := apiDo("GET", "/api/v1/ingredients", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var ings []struct {
		ID int `json:"id"`
	}
	parseJSON(t, body, &ings)
	if len(ings) == 0 {
		t.Fatal("no ingredients in DB")
	}

	// Create a new recipe
	resp2, body2 := apiDo("POST", "/api/v1/recipes", adminToken, map[string]any{
		"name":                    "E2E Временная Рецептура",
		"version":                 "1.0",
		"min_volume_l":            50,
		"max_volume_l":            200,
		"description":             "Удаляется в cleanup",
		"required_equipment_type": "VEH",
		"ingredients": []map[string]any{
			{"ingredient_id": ings[0].ID, "stage_key": "oil_phase", "percentage": 100.0},
		},
	})
	assertStatus(t, resp2, body2, http.StatusCreated)

	var created struct {
		RecipeCode string `json:"recipe_code"`
	}
	parseJSON(t, body2, &created)

	if created.RecipeCode == "" {
		t.Fatal("created recipe has empty code")
	}

	// Verify it's accessible
	resp3, body3 := apiDo("GET", "/api/v1/recipes/"+created.RecipeCode, adminToken, nil)
	assertStatus(t, resp3, body3, http.StatusOK)

	// Archive it
	resp4, body4 := apiDo("DELETE", "/api/v1/recipes/"+created.RecipeCode, adminToken, nil)
	assertStatus(t, resp4, body4, http.StatusNoContent)

	// After archiving, GET must return 409 (archived)
	resp5, body5 := apiDo("GET", "/api/v1/recipes/"+created.RecipeCode, adminToken, nil)
	if resp5.StatusCode != http.StatusConflict && resp5.StatusCode != http.StatusNotFound {
		t.Errorf("expected 409 or 404 after archive, got %d — %s", resp5.StatusCode, body5)
	}
}

func TestRecipe_Archive_NotFound(t *testing.T) {
	resp, body := apiDo("DELETE", "/api/v1/recipes/RC-GHOST-9999", adminToken, nil)
	assertStatus(t, resp, body, http.StatusNotFound)
}

func TestRecipe_GetIngredients_AdminOnly(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/ingredients", adminToken, nil)
	assertStatus(t, resp, body, http.StatusOK)

	var ings []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Unit string `json:"unit"`
	}
	parseJSON(t, body, &ings)

	if len(ings) == 0 {
		t.Error("expected at least one ingredient in DB")
	}
	for _, ing := range ings {
		if ing.Name == "" || ing.Unit == "" {
			t.Errorf("ingredient %d has empty name or unit", ing.ID)
		}
	}
}

func TestRecipe_Create_MissingName_Fails(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/recipes", adminToken, map[string]any{
		"version": "1.0",
		// name is missing
		"ingredients": []map[string]any{},
	})
	if resp.StatusCode == http.StatusCreated {
		t.Errorf("expected failure when name is missing, got 201 — %s", body)
	}
}
