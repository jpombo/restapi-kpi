package main

import (
	"log"
	"net/http"
	"time"

	"kpi-requirement-generator/config"
	"kpi-requirement-generator/database"
	"kpi-requirement-generator/handlers"
	"kpi-requirement-generator/ollama"
	"kpi-requirement-generator/routes"
	"kpi-requirement-generator/service"
)

func main() {
	// Carregar configuração
	cfg := config.LoadConfig()

	// Conectar ao banco
	db, err := database.NewDB(cfg)
	if err != nil {
		log.Fatalf("Erro ao conectar ao banco: %v", err)
	}
	defer db.Close()

	// Criar cliente Ollama com timeout
	ollamaClient := ollama.NewClientWithTimeout(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaTimeout)

	// Criar service
	kpiService := service.NewKPIService(db, ollamaClient)

	// Criar handler
	kpiHandler := handlers.NewKPIHandler(kpiService)

	// Configurar rotas
	router := routes.SetupRoutes(kpiHandler)

	// Configurar servidor
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 300 * time.Second, // Timeout longo para Ollama
		IdleTimeout:  60 * time.Second,
	}

	log.Println("Servidor iniciado na porta 8080")
	log.Println("Endpoints disponíveis:")
	log.Println("  POST   /api/v1/kpis                    - Criar KPI com metas e gerar requisitos")
	log.Println("  GET    /api/v1/kpis                    - Listar todos os KPIs")
	log.Println("  GET    /api/v1/kpis/{id}               - Buscar KPI por ID")
	log.Println("  POST   /api/v1/kpis/{id}/generate-requirements - Gerar requisitos para KPI existente")
	log.Println("  GET    /api/v1/health                  - Health check")

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}
}
