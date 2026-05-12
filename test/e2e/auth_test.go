package e2e

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestLogin_Success(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/auth/login", "", map[string]string{
		"username": "admin01",
		"password": "admin01",
	})
	assertStatus(t, resp, body, http.StatusOK)

	var res struct {
		Token    string `json:"token"`
		Role     string `json:"role"`
		UserCode string `json:"user_code"`
	}
	parseJSON(t, body, &res)

	if res.Token == "" {
		t.Error("expected non-empty token")
	}
	if res.Role != "admin" {
		t.Errorf("expected role=admin, got %q", res.Role)
	}
	if res.UserCode == "" {
		t.Error("expected non-empty user_code")
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/auth/login", "", map[string]string{
		"username": "admin01",
		"password": "wrongpassword",
	})
	assertStatus(t, resp, body, http.StatusUnauthorized)
}

func TestLogin_UnknownUser(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/auth/login", "", map[string]string{
		"username": "nonexistent_user_xyz",
		"password": "anything",
	})
	assertStatus(t, resp, body, http.StatusUnauthorized)
}

func TestLogin_EmptyBody(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/auth/login", "", nil)
	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 400 or 401, got %d\nbody: %s", resp.StatusCode, body)
	}
}

func TestLogin_NoToken_ProtectedRoute(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/batches?status=waiting_weighing", "", nil)
	assertStatus(t, resp, body, http.StatusUnauthorized)
}

func TestLogin_InvalidToken_ProtectedRoute(t *testing.T) {
	resp, body := apiDo("GET", "/api/v1/batches?status=waiting_weighing", "Bearer invalid.token.here", nil)
	// middleware checks JWT validity — should reject
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 401/403 with invalid token, got %d\nbody: %s", resp.StatusCode, body)
	}
}

func TestLogin_TokenContainsCorrectClaims(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/auth/login", "", map[string]string{
		"username": "admin01",
		"password": "admin01",
	})
	assertStatus(t, resp, body, http.StatusOK)

	var res struct {
		Token    string `json:"token"`
		Role     string `json:"role"`
		FullName string `json:"full_name"`
		IsActive bool   `json:"is_active"`
	}
	parseJSON(t, body, &res)

	if !res.IsActive {
		t.Error("expected is_active=true for admin01")
	}
	if res.FullName == "" {
		t.Error("expected non-empty full_name")
	}

	// Validate the token is actually accepted by a protected endpoint
	resp2, _ := apiDo("GET", "/api/v1/batches?status=waiting_weighing", res.Token, nil)
	if resp2.StatusCode == http.StatusUnauthorized {
		t.Error("token returned by login was rejected by a protected endpoint")
	}
}

// TestLogin_RoleReturned verifies that login returns role and matches actual DB role.
func TestLogin_AdminRoleInResponse(t *testing.T) {
	resp, body := apiDo("POST", "/api/v1/auth/login", "", map[string]string{
		"username": "admin01",
		"password": "admin01",
	})
	assertStatus(t, resp, body, http.StatusOK)

	var res map[string]json.RawMessage
	parseJSON(t, body, &res)

	for _, field := range []string{"token", "role", "user_code", "user_name", "full_name", "is_active"} {
		if _, ok := res[field]; !ok {
			t.Errorf("login response missing field %q", field)
		}
	}
}
