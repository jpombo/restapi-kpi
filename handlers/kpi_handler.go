package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

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
	if req.Nome == "" {
		respondWithError(w, http.StatusBadRequest, "Campo 'nome' é obrigatório")
		return
	}

	if req.IndicadorID == 0 {
		respondWithError(w, http.StatusBadRequest, "Campo 'indicador_id' é obrigatório")
		return
	}

	if req.TipoMeta == "" {
		respondWithError(w, http.StatusBadRequest, "Campo 'tipo_meta' é obrigatório")
		return
	}

	if req.Periodicidade == "" {
		respondWithError(w, http.StatusBadRequest, "Campo 'periodicidade' é obrigatório")
		return
	}

	// Processar KPI (insere no banco e chama Ollama)
	result, err := h.kpiService.CreateKPIWithMetasAndGenerateRequirements(&req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Erro ao processar KPI: "+err.Error())
		return
	}

	respondWithJSON(w, http.StatusCreated, result)
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
		respondWithError(w, http.StatusInternalServerError, "Erro ao gerar requisitos: "+err.Error())
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

// Funções auxiliares
func respondWithJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, status int, message string) {
	respondWithJSON(w, status, map[string]interface{}{
		"success": false,
		"error":   message,
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
