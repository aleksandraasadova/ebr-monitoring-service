package e2e

import (
	"net/http"
	"testing"
)

// createTestOperator creates an operator via admin token and returns their username and token.
// The service sets password = username, so we can login immediately.
func createTestOperator(t *testing.T, surname, name, fatherName string) (username, token string) {
	t.Helper()
	resp, body := apiDo("POST", "/api/v1/users", adminToken, map[string]string{
		"role":        "operator",
		"surname":     surname,
		"name":        name,
		"father_name": fatherName,
	})
	assertStatus(t, resp, body, http.StatusCreated)

	var res struct {
		UserCode string `json:"user_code"`
		UserName string `json:"user_name"`
	}
	parseJSON(t, body, &res)

	if res.UserCode == "" {
		t.Fatal("create user: empty user_code in response")
	}
	if res.UserName == "" {
		t.Fatal("create user: empty user_name in response")
	}

	// Password equals the generated username
	tok := mustLogin(t, res.UserName, res.UserName)
	return res.UserName, tok
}

func TestCreateUser_AdminCreatesOperator(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/users", adminToken, map[string]string{
		"role":        "operator",
		"surname":     "E2E",
		"name":        "Тест",
		"father_name": "Создания",
	})
	assertStatus(t, resp, body, http.StatusCreated)

	var res struct {
		UserCode string `json:"user_code"`
		UserName string `json:"user_name"`
	}
	parseJSON(t, body, &res)

	if res.UserCode == "" {
		t.Error("user_code must not be empty")
	}
	// Operator user_code starts with OP-
	if len(res.UserCode) < 3 || res.UserCode[:3] != "OP-" {
		t.Errorf("expected user_code to start with OP-, got %q", res.UserCode)
	}
}

func TestCreateUser_OperatorForbidden(t *testing.T) {
	// Create an operator first
	_, opToken := createTestOperator(t, "E2E", "Запрет", "Создания")

	// Operator tries to create another user → must be 403
	resp, body := apiDo("POST", "/api/v1/users", opToken, map[string]string{
		"role":        "operator",
		"surname":     "E2E",
		"name":        "Нельзя",
		"father_name": "Создавать",
	})
	assertStatus(t, resp, body, http.StatusForbidden)
}

func TestCreateUser_NoAuth_Rejected(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/users", "", map[string]string{
		"role":        "operator",
		"surname":     "E2E",
		"name":        "Без",
		"father_name": "Токена",
	})
	assertStatus(t, resp, body, http.StatusUnauthorized)
}

func TestCreateUser_MissingFields(t *testing.T) {
	// Missing father_name → service should reject
	resp, body := apiDo("POST", "/api/v1/users", adminToken, map[string]string{
		"role":    "operator",
		"surname": "E2E",
		"name":    "Только",
		// father_name is missing → will be empty string → len("") < 2
	})
	if resp.StatusCode == http.StatusCreated {
		t.Error("expected failure when father_name is missing, got 201")
	}
	_ = body
}

func TestCreateUser_OperatorCanLoginWithGeneratedCredentials(t *testing.T) {
	username, token := createTestOperator(t, "E2E", "Логин", "Проверка")

	if username == "" || token == "" {
		t.Fatal("expected non-empty username and token")
	}

	// Verify the token actually works on a protected endpoint
	resp, body := apiDo("GET", "/api/v1/batches?status=waiting_weighing", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("operator token rejected on GET /batches: %d — %s", resp.StatusCode, body)
	}
}

func TestCreateUser_OperatorCannotListRecipes(t *testing.T) {
	_, opToken := createTestOperator(t, "E2E", "БезРецептов", "Проверка")

	resp, body := apiDo("GET", "/api/v1/recipes", opToken, nil)
	assertStatus(t, resp, body, http.StatusForbidden)
}
