package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"kpi-requirement-generator/models"
	"kpi-requirement-generator/service"

	"github.com/gorilla/mux"
)

type KPIHandler struct {
	kpiService *service.KPIService
}

func NewKPIHandler(kpiService *service.KPIService) *KPIHandler {
	return &KPIHandler{
		kpiService: kpiService,
	}
}

// CreateKPIWithMetas handler para criar KPI, metas e gerar requisitos
func (h *KPIHandler) CreateKPIWithMetas(w http.ResponseWriter, r *http.Request) {
	var req models.CreateKPIRequest

	// Decodificar JSON
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "JSON inválido: "+err.Error())
		return
	}

	// Validar campos obrigatórios
	if err := validateRequest(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Processar KPI (insere no banco e chama Ollama)
	result, err := h.kpiService.CreateKPIWithMetasAndGenerateRequirements(&req)
	if err != nil {
		// Em caso de erro nos inserts, retorna HTTP 304 (Not Modified)
		respondWithError(w, http.StatusNotModified, "Falha ao inserir dados: "+err.Error())
		return
	}

	// Sucesso nos inserts - retorna HTTP 200 com os dados
	respondWithJSON(w, http.StatusOK, result)
}

// GetKPI handler para buscar um KPI por ID
func (h *KPIHandler) GetKPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	kpi, metas, err := h.kpiService.GetKPIWithMetas(uint(id))
	if err != nil {
		respondWithError(w, http.StatusNotFound, "KPI não encontrado: "+err.Error())
		return
	}

	response := models.CompleteKPIResponse{
		KPI:     convertToKPIResponse(kpi),
		Metas:   convertToMetaResponses(metas),
		Success: true,
	}

	respondWithJSON(w, http.StatusOK, response)
}

// GenerateRequirementsForKPI handler para gerar requisitos para um KPI existente
func (h *KPIHandler) GenerateRequirementsForKPI(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "ID inválido")
		return
	}

	requirements, err := h.kpiService.GenerateRequirementsForExistingKPI(uint(id))
	if err != nil {
		respondWithError(w, http.StatusNotModified, "Erro ao gerar requisitos: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success":    true,
		"kpi_id":     id,
		"requisitos": requirements,
		"message":    "Requisitos gerados com sucesso",
	})
}

// GetAllKPIs handler para listar todos os KPIs
func (h *KPIHandler) GetAllKPIs(w http.ResponseWriter, r *http.Request) {
	kpis, err := h.kpiService.GetAllKPIs()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Erro ao buscar KPIs: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    kpis,
		"count":   len(kpis),
	})
}

// HealthCheck handler
func (h *KPIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "healthy",
		"service": "kpi-requirement-generator",
	})
}

// validateRequest valida os campos obrigatórios
func validateRequest(req *models.CreateKPIRequest) error {
	if req.Nome == "" {
		return fmt.Errorf("campo 'nome' é obrigatório")
	}

	if req.IndicadorID == 0 {
		return fmt.Errorf("campo 'indicador_id' é obrigatório")
	}

	if req.TipoMeta == "" {
		return fmt.Errorf("campo 'tipo_meta' é obrigatório")
	}

	if req.Periodicidade == "" {
		return fmt.Errorf("campo 'periodicidade' é obrigatório")
	}

	return nil
}

// Funções auxiliares
func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Erro ao encode JSON: %v", err)
	}
}

func respondWithError(w http.ResponseWriter, status int, message string) {
	respondWithJSON(w, status, map[string]interface{}{
		"success":   false,
		"error":     message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func convertToKPIResponse(kpi *models.KPI) *models.KPIResponse {
	return &models.KPIResponse{
		ID:            kpi.ID,
		Nome:          kpi.Nome,
		Descricao:     kpi.Descricao,
		IndicadorID:   kpi.IndicadorID,
		TipoMeta:      kpi.TipoMeta,
		Periodicidade: kpi.Periodicidade,
		CreatedAt:     kpi.DataCriacao,
	}
}

func convertToMetaResponses(metas []models.Meta) []models.MetaResponse {
	var responses []models.MetaResponse
	for _, m := range metas {
		responses = append(responses, models.MetaResponse{
			ID:                  m.ID,
			Ano:                 m.Ano,
			PeriodoReferencia:   m.PeriodoReferencia,
			ValorMetaNumerica:   m.ValorMetaNumerica,
			ValorMetaPercentual: m.ValorMetaPercentual,
			ValorMetaMin:        m.ValorMetaMin,
			ValorMetaMax:        m.ValorMetaMax,
			TipoMetaRealizado:   m.TipoMetaRealizado,
			Observacao:          m.Observacao,
		})
	}
	return responses
}
