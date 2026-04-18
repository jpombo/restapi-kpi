package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kpi-requirement-generator/config"
	"kpi-requirement-generator/database"
	"kpi-requirement-generator/ollama"
	"kpi-requirement-generator/service"
)

func main() {
	var kpiID uint
	var allKPIs bool

	flag.UintVar(&kpiID, "kpi-id", 0, "ID do KPI a ser processado")
	flag.BoolVar(&allKPIs, "all", false, "Processar todos os KPIs ativos")
	flag.Parse()

	if kpiID == 0 && !allKPIs {
		log.Fatal("É necessário informar -kpi-id ou -all")
	}

	// Carregar configuração
	cfg := config.LoadConfig()

	// Conectar ao banco
	db, err := database.NewDB(cfg)
	if err != nil {
		log.Fatalf("Erro ao conectar ao banco: %v", err)
	}
	defer db.Close()

	// Criar cliente Ollama com timeout configurável
	ollamaClient := ollama.NewClientWithTimeout(cfg.OllamaURL, cfg.OllamaModel, cfg.OllamaTimeout)

	// Criar service
	kpiService := service.NewKPIService(db, ollamaClient)

	// Contexto com timeout global (ex: 10 minutos)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Configurar graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Recebido sinal de interrupção, finalizando...")
		cancel()
		os.Exit(0)
	}()

	// Processar KPI com contexto
	if allKPIs {
		log.Println("Processando todos os KPIs ativos...")
		// Implementar processamento em lote
	} else {
		log.Printf("Processando KPI ID: %d (timeout: %d segundos)", kpiID, cfg.OllamaTimeout)

		// Criar channel para receber resultado
		done := make(chan error, 1)

		go func() {
			done <- kpiService.ProcessKPI(kpiID)
		}()

		// Aguardar resultado ou timeout
		select {
		case err := <-done:
			if err != nil {
				log.Fatalf("Erro ao processar KPI: %v", err)
			}
			log.Println("Processamento concluído com sucesso!")
		case <-ctx.Done():
			log.Fatal("Timeout global excedido ao processar KPI")
		}
	}
}
