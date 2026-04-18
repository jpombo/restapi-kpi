package service

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"kpi-requirement-generator/database"
	"kpi-requirement-generator/models"
	"kpi-requirement-generator/ollama"
)

type KPIService struct {
	db           *database.DB
	ollamaClient *ollama.Client
}

func NewKPIService(db *database.DB, ollamaClient *ollama.Client) *KPIService {
	return &KPIService{
		db:           db,
		ollamaClient: ollamaClient,
	}
}

// ProcessKPI processa um KPI, gera requisitos via Ollama e salva no banco
func (s *KPIService) ProcessKPI(kpiID uint) error {
	log.Printf("Processando KPI ID: %d", kpiID)

	// 1. Buscar KPI do banco
	kpi, err := s.db.GetKPIWithIndicador(kpiID)
	if err != nil {
		return fmt.Errorf("erro ao buscar KPI: %w", err)
	}

	// 2. Preparar dados para enviar ao Ollama
	kpiRequest := s.prepareKPIRequest(kpi)
	kpiJSON, err := json.Marshal(kpiRequest)
	if err != nil {
		return fmt.Errorf("erro ao serializar KPI: %w", err)
	}

	log.Printf("Enviando para Ollama: %s", string(kpiJSON))

	// 3. Chamar Ollama
	response, err := s.ollamaClient.GenerateRequirements(string(kpiJSON))
	if err != nil {
		return fmt.Errorf("erro ao chamar Ollama: %w", err)
	}

	log.Printf("Resposta recebida do Ollama (tamanho: %d bytes)", len(response))

	// 4. Parse da resposta JSON
	requirements, err := s.parseRequirementsResponse(response)
	if err != nil {
		return fmt.Errorf("erro ao parsear resposta: %w", err)
	}

	// 5. Salvar requisitos técnicos
	for _, req := range requirements.RequisitosTecnicos {
		if err := s.saveRequisitoTecnico(kpiID, req); err != nil {
			log.Printf("Erro ao salvar requisito %s: %v", req.Codigo, err)
			continue
		}
		log.Printf("Requisito técnico %s salvo com sucesso", req.Codigo)
	}

	// 6. Salvar requisitos não funcionais
	for _, req := range requirements.RequisitosNaoFuncionais {
		if err := s.saveRequisitoNaoFuncional(kpiID, req); err != nil {
			log.Printf("Erro ao salvar requisito %s: %v", req.Codigo, err)
			continue
		}
		log.Printf("Requisito não funcional %s salvo com sucesso", req.Codigo)
	}

	// 7. Salvar metas sugeridas
	for _, meta := range requirements.MetasSugeridas {
		if err := s.saveMeta(kpiID, meta); err != nil {
			log.Printf("Erro ao salvar meta: %v", err)
			continue
		}
		log.Printf("Meta salva com sucesso para período %s", meta.PeriodoReferencia)
	}

	log.Printf("KPI %d processado com sucesso!", kpiID)
	return nil
}

func (s *KPIService) prepareKPIRequest(kpi *models.KPI) *models.KPIRequest {
	req := &models.KPIRequest{
		ID:               kpi.ID,
		Nome:             kpi.Nome,
		Descricao:        kpi.Descricao,
		TipoMeta:         kpi.TipoMeta,
		Periodicidade:    kpi.Periodicidade,
		FormulaCalculo:   kpi.FormulaCalculo,
		UnidadeMedidaKPI: kpi.UnidadeMedidaKPI,
	}

	if kpi.Indicador != nil {
		req.Indicador.ID = kpi.Indicador.ID
		req.Indicador.Nome = kpi.Indicador.Nome
		req.Indicador.UnidadeMedida = kpi.Indicador.UnidadeMedida
	}

	return req
}

func (s *KPIService) parseRequirementsResponse(response string) (*models.GeneratedRequirements, error) {
	// Extrai o JSON da resposta (pode conter texto antes/depois)
	inicio := strings.Index(response, "{")
	fim := strings.LastIndex(response, "}")

	if inicio == -1 || fim == -1 {
		return nil, fmt.Errorf("resposta não contém JSON válido")
	}

	jsonStr := response[inicio : fim+1]

	var requirements models.GeneratedRequirements
	if err := json.Unmarshal([]byte(jsonStr), &requirements); err != nil {
		return nil, fmt.Errorf("erro ao decodificar JSON: %w\nResposta: %s", err, jsonStr)
	}

	return &requirements, nil
}

func (s *KPIService) saveRequisitoTecnico(kpiID uint, req struct {
	Codigo               string `json:"codigo"`
	Titulo               string `json:"titulo"`
	Descricao            string `json:"descricao"`
	Prioridade           string `json:"prioridade"`
	ComplexidadeEstimada string `json:"complexidade_estimada"`
}) error {
	requisito := &models.RequisitoKPI{
		KPIID:                kpiID,
		TipoRequisitoID:      1, // TECNICO
		Codigo:               req.Codigo,
		Titulo:               req.Titulo,
		Descricao:            req.Descricao,
		Prioridade:           req.Prioridade,
		ComplexidadeEstimada: req.ComplexidadeEstimada,
		Status:               "APROVADO",
		CriadoPor:            "OLLAMA_GENERATOR",
	}

	return s.db.SaveRequisito(requisito)
}

func (s *KPIService) saveRequisitoNaoFuncional(kpiID uint, req struct {
	Codigo               string `json:"codigo"`
	Titulo               string `json:"titulo"`
	Descricao            string `json:"descricao"`
	Categoria            string `json:"categoria"`
	Prioridade           string `json:"prioridade"`
	ComplexidadeEstimada string `json:"complexidade_estimada"`
}) error {
	// Busca ID da categoria
	categoriaID, err := s.db.GetCategoriaNaoFuncionalID(req.Categoria)
	if err != nil {
		return err
	}

	requisito := &models.RequisitoKPI{
		KPIID:                   kpiID,
		TipoRequisitoID:         2, // NAO_FUNCIONAL
		Codigo:                  req.Codigo,
		Titulo:                  req.Titulo,
		Descricao:               req.Descricao,
		CategoriaNaoFuncionalID: categoriaID,
		Prioridade:              req.Prioridade,
		ComplexidadeEstimada:    req.ComplexidadeEstimada,
		Status:                  "APROVADO",
		CriadoPor:               "OLLAMA_GENERATOR",
	}

	return s.db.SaveRequisito(requisito)
}

func (s *KPIService) saveMeta(kpiID uint, meta struct {
	PeriodoReferencia string `json:"periodo_referencia"`
	TipoMeta          string `json:"tipo_meta"`
	Valor             string `json:"valor"`
	Observacao        string `json:"observacao"`
}) error {
	metaModel := &models.Meta{
		KPIID:             kpiID,
		Ano:               uint(time.Now().Year()),
		PeriodoReferencia: &meta.PeriodoReferencia,
		TipoMetaRealizado: meta.TipoMeta,
		Observacao:        &meta.Observacao,
	}

	// Parse do valor baseado no tipo
	switch meta.TipoMeta {
	case "NUMERICA":
		val, err := strconv.ParseFloat(strings.Replace(meta.Valor, "R$", "", -1), 64)
		if err != nil {
			return err
		}
		metaModel.ValorMetaNumerica = &val

	case "PERCENTUAL":
		val, err := strconv.ParseFloat(strings.Replace(meta.Valor, "%", "", -1), 64)
		if err != nil {
			return err
		}
		metaModel.ValorMetaPercentual = &val

	case "INTERVALO_NUM":
		partes := strings.Split(meta.Valor, " a ")
		if len(partes) == 2 {
			min, err := strconv.ParseFloat(partes[0], 64)
			if err != nil {
				return err
			}
			max, err := strconv.ParseFloat(partes[1], 64)
			if err != nil {
				return err
			}
			metaModel.ValorMetaMin = &min
			metaModel.ValorMetaMax = &max
		}
	}

	return s.db.SaveMeta(metaModel)
}

// service/kpi_service.go - Adicionar retry
func (s *KPIService) ProcessKPIWithRetry(kpiID uint, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("Tentativa %d de %d para KPI %d", attempt, maxRetries, kpiID)

		err := s.ProcessKPI(kpiID)
		if err == nil {
			return nil
		}

		lastErr = err

		// Verificar se é erro de timeout
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			waitTime := time.Duration(attempt*30) * time.Second
			log.Printf("Timeout detectado, aguardando %v antes de tentar novamente...", waitTime)
			time.Sleep(waitTime)
			continue
		}

		// Outros erros não devem ser retentados
		return err
	}

	return fmt.Errorf("falha após %d tentativas: %w", maxRetries, lastErr)
}
