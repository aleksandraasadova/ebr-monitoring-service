package e2e

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/repository"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
	transport "github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/http"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/wsserver"
)

var (
	srv            *httptest.Server
	db             *sql.DB
	adminToken     string
	testRecipeCode string
)

func TestMain(m *testing.M) {
	// Load .env from repo root (two levels up from test/e2e/).
	// Ignore error — the env might already be set (CI, docker, etc.).
	_ = godotenv.Load("../../.env")

	// JWT_SECRET must be set before creating the server.
	if os.Getenv("JWT_SECRET") == "" {
		os.Setenv("JWT_SECRET", "e2e-test-secret-key")
	}

	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		panic("DB_URL is not set — check your .env file and run: source .env")
	}

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		panic("cannot open db: " + err.Error())
	}
	if err = db.Ping(); err != nil {
		panic("cannot connect to db — is docker-compose up? err: " + err.Error())
	}

	hub := wsserver.NewHub()
	go hub.Run()

	userRepo      := repository.NewUserRepo(db)
	recipeRepo    := repository.NewRecipeRepo(db)
	batchRepo     := repository.NewBatchRepo(db)
	telemetryRepo := repository.NewTelemetryRepo(db)
	processRepo   := repository.NewProcessRepo(db)
	eventRepo     := repository.NewEventRepo(db)
	reportRepo    := repository.NewReportRepo(db)
	analyticsRepo := repository.NewAnalyticsRepo(db)

	telemetrySvc := service.NewTelemetryService(telemetryRepo)
	telemetrySvc.SetBroadcaster(hub)

	userSvc    := service.NewUserService(userRepo)
	authSvc    := service.NewAuthService(userRepo)
	recipeSvc  := service.NewRecipeService(recipeRepo)
	batchSvc   := service.NewBatchService(batchRepo, recipeRepo, userRepo)
	processSvc := service.NewProcessService(processRepo, eventRepo, userRepo, telemetrySvc)
	reportSvc  := service.NewReportService(reportRepo, batchRepo, processRepo, eventRepo, telemetryRepo, recipeRepo)

	mux := transport.NewRouter(transport.RouterDeps{
		WebDir:         "../../web",
		UserService:    userSvc,
		AuthService:    authSvc,
		RecipeService:  recipeSvc,
		BatchService:   batchSvc,
		TelemetrySvc:   telemetrySvc,
		ProcessService: processSvc,
		ReportService:  reportSvc,
		Hub:            hub,
		AnalyticsRepo:  analyticsRepo,
	})

	srv = httptest.NewServer(mux)
	defer srv.Close()

	adminToken = mustLogin(t_stub(), "admin01", "admin01")
	testRecipeCode = mustCreateTestRecipe()

	code := m.Run()

	cleanupTestData()
	os.Exit(code)
}

// t_stub returns a minimal *testing.T for use outside of test functions.
// Panics are acceptable here since test setup failures are fatal anyway.
func t_stub() *testing.T { return &testing.T{} }

// mustLogin logs in and returns JWT token. Panics on failure (test setup error).
func mustLogin(_ *testing.T, username, password string) string {
	resp, body := apiDo("POST", "/api/v1/auth/login", "", map[string]string{
		"username": username,
		"password": password,
	})
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Sprintf("mustLogin(%s) failed: %d — %s", username, resp.StatusCode, string(body)))
	}
	var res struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		panic("mustLogin: cannot decode token: " + err.Error())
	}
	if res.Token == "" {
		panic("mustLogin: empty token for " + username)
	}
	return res.Token
}

// mustCreateTestRecipe creates the recipe used by batch tests and returns its code.
func mustCreateTestRecipe() string {
	// First check if ingredients are available
	resp, body := apiDo("GET", "/api/v1/ingredients", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		panic("cannot get ingredients: " + string(body))
	}
	var ings []struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(body, &ings); err != nil || len(ings) < 2 {
		panic("cannot parse ingredients or DB has none: " + string(body))
	}

	// Use first two ingredients for the test recipe
	resp2, body2 := apiDo("POST", "/api/v1/recipes", adminToken, map[string]any{
		"name":                    "Тестовый крем E2E",
		"version":                 "test",
		"min_volume_l":            100,
		"max_volume_l":            500,
		"description":             "Только для автотестов",
		"required_equipment_type": "VEH",
		"ingredients": []map[string]any{
			{"ingredient_id": ings[0].ID, "stage_key": "oil_phase", "percentage": 30.0},
			{"ingredient_id": ings[1].ID, "stage_key": "water_phase", "percentage": 70.0},
		},
	})
	if resp2.StatusCode != http.StatusCreated {
		panic("cannot create test recipe: " + string(body2))
	}
	var res struct {
		RecipeCode string `json:"recipe_code"`
	}
	if err := json.Unmarshal(body2, &res); err != nil || res.RecipeCode == "" {
		panic("cannot parse recipe_code from response: " + string(body2))
	}
	return res.RecipeCode
}

// cleanupTestData removes all data created during test run.
func cleanupTestData() {
	// Remove the test recipe and any batches that used it
	var recipeID int
	db.QueryRow(`SELECT id FROM recipes WHERE recipe_code = $1`, testRecipeCode).Scan(&recipeID)
	if recipeID > 0 {
		// Batches referencing the recipe (cascade deletes weighing_log, batch_stages, events, batch_reports, telemetry)
		db.Exec(`DELETE FROM batches WHERE recipe_id = $1`, recipeID)
		db.Exec(`DELETE FROM recipe_ingredients WHERE recipe_id = $1`, recipeID)
		db.Exec(`DELETE FROM recipes WHERE id = $1`, recipeID)
	}
	// Remove any test users created during tests (identified by full_name prefix)
	db.Exec(`DELETE FROM users WHERE full_name LIKE 'E2E %'`)
}

// apiDo performs an HTTP request to the test server.
// Returns response and body bytes. Body is always fully read and closed.
func apiDo(method, path, token string, payload any) (*http.Response, []byte) {
	var r io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			panic("apiDo marshal: " + err.Error())
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		panic("apiDo new request: " + err.Error())
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic("apiDo do: " + err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

// parseJSON decodes JSON body into dest. Calls t.Fatalf on error.
func parseJSON(t *testing.T, body []byte, dest any) {
	t.Helper()
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("cannot parse JSON: %v\nbody: %s", err, string(body))
	}
}

// assertStatus calls t.Errorf if the response status doesn't match expected.
func assertStatus(t *testing.T, resp *http.Response, body []byte, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Errorf("expected HTTP %d, got %d\nbody: %s", want, resp.StatusCode, string(body))
	}
}
