package transport

import (
	"encoding/json"
	"net/http"

	"github.com/aleksandraasadova/ebr-monitoring-service/internal/domain"
	"github.com/aleksandraasadova/ebr-monitoring-service/internal/service"
)

func GetRecipeByCodeHandler(rs *service.RecipeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := r.PathValue("code")

		resp, err := rs.GetByCode(r.Context(), code)
		if err != nil {
			if err == domain.ErrRecipeNotFound {
				http.Error(w, "recipe not found", http.StatusNotFound)
				return
			}
			if err == domain.ErrRecipeArchived {
				http.Error(w, "recipe archived", http.StatusConflict)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}
}
