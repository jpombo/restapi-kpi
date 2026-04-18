package routes

import (
	"kpi-requirement-generator/handlers"
	"net/http"

	"github.com/gorilla/mux"
)

func SetupRoutes(handler *handlers.KPIHandler) *mux.Router {
	router := mux.NewRouter()

	// Middleware global (opcional)
	router.Use(contentTypeMiddleware)

	// Rotas de API
	api := router.PathPrefix("/api/v1").Subrouter()

	// Health check
	api.HandleFunc("/health", handler.HealthCheck).Methods("GET")

	// Rotas de KPI
	api.HandleFunc("/kpis", handler.CreateKPIWithMetas).Methods("POST")
	api.HandleFunc("/kpis", handler.GetAllKPIs).Methods("GET")
	api.HandleFunc("/kpis/{id}", handler.GetKPI).Methods("GET")
	api.HandleFunc("/kpis/{id}/generate-requirements", handler.GenerateRequirementsForKPI).Methods("POST")

	return router
}

// Middleware para garantir Content-Type
func contentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
