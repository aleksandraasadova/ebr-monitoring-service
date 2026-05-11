package transport

import (
	"net/http"
	"path/filepath"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/transport/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type RouterDeps struct {
	WebDir        string
	UserService   *service.UserService
	AuthService   *service.AuthService
	RecipeService *service.RecipeService
	BatchService  *service.BatchService
	TelemetrySvc  *service.TelemetryService
}

func NewRouter(d RouterDeps) *http.ServeMux {
	m := http.NewServeMux()

	fs := http.FileServer(http.Dir(d.WebDir))
	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		if r.URL.Path == "/" {
			http.ServeFile(w, r, filepath.Join(d.WebDir, "login.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})

	m.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	authH := NewAuthHandler(d.AuthService)
	userH := NewUserHandler(d.UserService)
	recipeH := NewRecipeHandler(d.RecipeService)
	batchH := NewBatchHandler(d.BatchService)
	telemetryH := NewTelemetryHandler(d.TelemetrySvc)

	m.HandleFunc("POST /api/v1/auth/login", authH.Login)

	m.Handle("POST /api/v1/users",
		middleware.JWT(middleware.RequireRole("admin")(http.HandlerFunc(userH.Create))))

	m.Handle("GET /api/v1/recipes/{code}",
		middleware.JWT(middleware.RequireRole("admin", "operator")(http.HandlerFunc(recipeH.GetByCode))))

	m.Handle("POST /api/v1/batches",
		middleware.JWT(middleware.RequireRole("operator")(http.HandlerFunc(batchH.Create))))

	m.Handle("GET /api/v1/batches",
		middleware.JWT(middleware.RequireRole("admin", "operator")(http.HandlerFunc(batchH.ListByStatus))))

	m.Handle("GET /api/v1/batches/{code}/weighing",
		middleware.JWT(middleware.RequireRole("admin", "operator")(http.HandlerFunc(batchH.GetWeighingLog))))

	m.Handle("POST /api/v1/batches/{code}/weighing/start",
		middleware.JWT(middleware.RequireRole("operator")(http.HandlerFunc(batchH.StartWeighing))))

	m.Handle("POST /api/v1/batches/{code}/weighing/{itemID}/confirm",
		middleware.JWT(middleware.RequireRole("operator")(http.HandlerFunc(batchH.ConfirmWeighingItem))))

	m.Handle("GET /api/v1/telemetry/weight/current",
		middleware.JWT(middleware.RequireRole("admin", "operator")(http.HandlerFunc(telemetryH.CurrentWeight))))

	return m
}
