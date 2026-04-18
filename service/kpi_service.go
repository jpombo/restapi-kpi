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

// CreateKPIWithMetasAndGenerateRequirements cria KPI, metas e gera requisitos via Ollama
func (s *KPIService) CreateKPIWithMetasAndGenerateRequirements(req *models.CreateKPIRequest) (*models.CompleteKPIResponse, error) {
	log.Printf("Iniciando criação do KPI: %s", req.Nome)

	// 1. Validar se indicador existe
	indicador, err := s.db.GetIndicadorByID(req.IndicadorID)
	if err != nil {
		return nil, fmt.Errorf("indicador não encontrado: %w", err)
	}

	// 2. Criar KPI no banco
	kpi := &models.KPI{
		IndicadorID:      req.IndicadorID,
		Nome:             req.Nome,
		Descricao:        req.Descricao,
		TipoMeta:         req.TipoMeta,
		Periodicidade:    req.Periodicidade,
		FormulaCalculo:   req.FormulaCalculo,
		UnidadeMedidaKPI: req.UnidadeMedidaKPI,
		Ativo:            true,
		DataCriacao:      time.Now(),
	}

	kpiID, err := s.db.CreateKPI(kpi)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar KPI: %w", err)
	}

	log.Printf("KPI criado com ID: %d", kpiID)
	kpi.ID = kpiID

	// 3. Inserir metas
	var metasCriadas []models.Meta
	for _, metaReq := range req.Metas {
		meta := &models.Meta{
			KPIID:               kpiID,
			Ano:                 metaReq.Ano,
			PeriodoReferencia:   &metaReq.PeriodoReferencia,
			ValorMetaNumerica:   metaReq.ValorMetaNumerica,
			ValorMetaPercentual: metaReq.ValorMetaPercentual,
			ValorMetaMin:        metaReq.ValorMetaMin,
			ValorMetaMax:        metaReq.ValorMetaMax,
			TipoMetaRealizado:   metaReq.TipoMetaRealizado,
			Observacao:          metaReq.Observacao,
		}

		if err := s.db.CreateMeta(meta); err != nil {
			log.Printf("Erro ao criar meta: %v", err)
			continue
		}

		metasCriadas = append(metasCriadas, *meta)
	}

	log.Printf("%d metas inseridas", len(metasCriadas))

	// 4. Preparar dados para enviar ao Ollama
	kpiWithIndicador := &models.KPI{
		ID:               kpiID,
		Nome:             req.Nome,
		Descricao:        req.Descricao,
		TipoMeta:         req.TipoMeta,
		Periodicidade:    req.Periodicidade,
		FormulaCalculo:   req.FormulaCalculo,
		UnidadeMedidaKPI: req.UnidadeMedidaKPI,
		Indicador:        indicador,
	}

	// 5. Chamar Ollama para gerar requisitos
	log.Println("Chamando Ollama para gerar requisitos...")
	requirements, err := s.generateRequirementsFromKPI(kpiWithIndicador)
	if err != nil {
		log.Printf("Erro ao gerar requisitos via Ollama: %v", err)
		// Não falha a criação do KPI, apenas retorna sem requisitos
		return &models.CompleteKPIResponse{
			KPI:     convertToKPIResponse(kpi),
			Metas:   convertToMetaResponses(metasCriadas),
			Success: true,
			Message: "KPI criado, mas falha ao gerar requisitos: " + err.Error(),
		}, nil
	}

	// 6. Salvar requisitos gerados no banco
	if err := s.saveGeneratedRequirements(kpiID, requirements); err != nil {
		log.Printf("Erro ao salvar requisitos: %v", err)
	}

	// 7. Montar resposta
	response := &models.CompleteKPIResponse{
		KPI:        convertToKPIResponse(kpi),
		Requisitos: requirements,
		Metas:      convertToMetaResponses(metasCriadas),
		Success:    true,
		Message:    "KPI, metas e requisitos criados com sucesso",
	}

	log.Printf("KPI %s processado com sucesso!", req.Nome)
	return response, nil
}

// GenerateRequirementsForExistingKPI gera requisitos para um KPI existente
func (s *KPIService) GenerateRequirementsForExistingKPI(kpiID uint) (*models.GeneratedRequirements, error) {
	// Buscar KPI e indicador
	kpi, err := s.db.GetKPIWithIndicador(kpiID)
	if err != nil {
		return nil, fmt.Errorf("KPI não encontrado: %w", err)
	}

	// Gerar requisitos via Ollama
	requirements, err := s.generateRequirementsFromKPI(kpi)
	if err != nil {
		return nil, err
	}

	// Salvar requisitos
	if err := s.saveGeneratedRequirements(kpiID, requirements); err != nil {
		return nil, fmt.Errorf("erro ao salvar requisitos: %w", err)
	}

	return requirements, nil
}

// generateRequirementsFromKPI prepara os dados e chama o Ollama
func (s *KPIService) generateRequirementsFromKPI(kpi *models.KPI) (*models.GeneratedRequirements, error) {
	// Preparar request para Ollama
	kpiRequest := &models.KPIRequest{
		ID:               kpi.ID,
		Nome:             kpi.Nome,
		Descricao:        kpi.Descricao,
		TipoMeta:         kpi.TipoMeta,
		Periodicidade:    kpi.Periodicidade,
		FormulaCalculo:   kpi.FormulaCalculo,
		UnidadeMedidaKPI: kpi.UnidadeMedidaKPI,
	}

	if kpi.Indicador != nil {
		kpiRequest.Indicador.ID = kpi.Indicador.ID
		kpiRequest.Indicador.Nome = kpi.Indicador.Nome
		kpiRequest.Indicador.UnidadeMedida = kpi.Indicador.UnidadeMedida
	}

	kpiJSON, err := json.Marshal(kpiRequest)
	if err != nil {
		return nil, fmt.Errorf("erro ao serializar KPI: %w", err)
	}

	log.Printf("Enviando para Ollama: %s", string(kpiJSON))

	// Chamar Ollama
	response, err := s.ollamaClient.GenerateRequirements(string(kpiJSON))
	if err != nil {
		return nil, fmt.Errorf("erro ao chamar Ollama: %w", err)
	}

	// Parse da resposta
	requirements, err := s.parseRequirementsResponse(response)
	if err != nil {
		return nil, fmt.Errorf("erro ao parsear resposta: %w", err)
	}

	return requirements, nil
}

// parseRequirementsResponse extrai o JSON da resposta do Ollama
func (s *KPIService) parseRequirementsResponse(response string) (*models.GeneratedRequirements, error) {
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

// saveGeneratedRequirements salva os requisitos no banco
func (s *KPIService) saveGeneratedRequirements(kpiID uint, requirements *models.GeneratedRequirements) error {
	// Salvar requisitos técnicos
	for _, req := range requirements.RequisitosTecnicos {
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
			DataCriacao:          time.Now(),
		}

		if err := s.db.SaveRequisito(requisito); err != nil {
			log.Printf("Erro ao salvar requisito %s: %v", req.Codigo, err)
		}
	}

	// Salvar requisitos não funcionais
	for _, req := range requirements.RequisitosNaoFuncionais {
		categoriaID, _ := s.db.GetCategoriaNaoFuncionalID(req.Categoria)

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
			DataCriacao:             time.Now(),
		}

		if err := s.db.SaveRequisito(requisito); err != nil {
			log.Printf("Erro ao salvar requisito %s: %v", req.Codigo, err)
		}
	}

	return nil
}

// GetKPIWithMetas busca KPI e suas metas
func (s *KPIService) GetKPIWithMetas(kpiID uint) (*models.KPI, []models.Meta, error) {
	return s.db.GetKPIByID(kpiID)
}

// GetAllKPIs busca todos os KPIs ativos
func (s *KPIService) GetAllKPIs() ([]models.KPIResponse, error) {
	query := `
        SELECT id, nome, descricao, indicador_id, tipo_meta, periodicidade, data_criacao
        FROM kpi
        WHERE ativo = TRUE
        ORDER BY data_criacao DESC
    `

	rows, err := s.db.Conn.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var kpis []models.KPIResponse
	for rows.Next() {
		var kpi models.KPIResponse
		err := rows.Scan(
			&kpi.ID, &kpi.Nome, &kpi.Descricao, &kpi.IndicadorID,
			&kpi.TipoMeta, &kpi.Periodicidade, &kpi.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		kpis = append(kpis, kpi)
	}

	return kpis, nil
}

// Função auxiliar
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
