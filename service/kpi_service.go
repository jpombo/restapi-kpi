package service

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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
// Retorna erro APENAS se houver falha nos inserts (rollback completo)
func (s *KPIService) CreateKPIWithMetasAndGenerateRequirements(req *models.CreateKPIRequest) (*models.CompleteKPIResponse, error) {
	log.Printf("Iniciando criação do KPI: %s", req.Nome)

	// Iniciar transação
	tx, err := s.db.Conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("erro ao iniciar transação: %w", err)
	}

	// Garantir rollback em caso de erro
	defer func() {
		if err != nil {
			tx.Rollback()
			log.Printf("Transação cancelada (rollback): %v", err)
		}
	}()

	// 1. Validar se indicador existe
	var indicadorID int
	err = tx.QueryRow("SELECT id FROM indicador WHERE id = ?", req.IndicadorID).Scan(&indicadorID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("indicador com ID %d não encontrado", req.IndicadorID)
		}
		return nil, fmt.Errorf("erro ao verificar indicador: %w", err)
	}

	// 2. Inserir KPI
	kpiQuery := `
        INSERT INTO kpi 
        (indicador_id, nome, descricao, tipo_meta, periodicidade, 
         formula_calculo, unidade_medida_kpi, ativo, data_criacao)
        VALUES (?, ?, ?, ?, ?, ?, ?, TRUE, NOW())
    `

	result, err := tx.Exec(
		kpiQuery,
		req.IndicadorID,
		req.Nome,
		req.Descricao,
		req.TipoMeta,
		req.Periodicidade,
		req.FormulaCalculo,
		req.UnidadeMedidaKPI,
	)

	if err != nil {
		return nil, fmt.Errorf("erro ao inserir KPI: %w", err)
	}

	kpiID, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("erro ao obter ID do KPI: %w", err)
	}

	log.Printf("KPI criado com ID: %d", kpiID)

	// 3. Inserir metas
	var metasCriadas []models.Meta
	for _, metaReq := range req.Metas {
		metaQuery := `
            INSERT INTO meta 
            (kpi_id, ano, periodo_referencia, valor_meta_numerica, valor_meta_percentual,
             valor_meta_min, valor_meta_max, tipo_meta_realizado, observacao)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
        `

		_, err := tx.Exec(
			metaQuery,
			kpiID,
			metaReq.Ano,
			metaReq.PeriodoReferencia,
			metaReq.ValorMetaNumerica,
			metaReq.ValorMetaPercentual,
			metaReq.ValorMetaMin,
			metaReq.ValorMetaMax,
			metaReq.TipoMetaRealizado,
			metaReq.Observacao,
		)

		if err != nil {
			return nil, fmt.Errorf("erro ao inserir meta: %w", err)
		}

		metasCriadas = append(metasCriadas, models.Meta{
			KPIID:               uint(kpiID),
			Ano:                 metaReq.Ano,
			PeriodoReferencia:   &metaReq.PeriodoReferencia,
			ValorMetaNumerica:   metaReq.ValorMetaNumerica,
			ValorMetaPercentual: metaReq.ValorMetaPercentual,
			ValorMetaMin:        metaReq.ValorMetaMin,
			ValorMetaMax:        metaReq.ValorMetaMax,
			TipoMetaRealizado:   metaReq.TipoMetaRealizado,
			Observacao:          metaReq.Observacao,
		})
	}

	log.Printf("%d metas inseridas", len(metasCriadas))

	// 4. COMMIT da transação (apenas após todas as inserções bem-sucedidas)
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("erro ao commitar transação: %w", err)
	}

	log.Printf("Transação commitada com sucesso para KPI %d", kpiID)

	// 5. Buscar indicador completo (fora da transação)
	indicador, err := s.db.GetIndicadorByID(req.IndicadorID)
	if err != nil {
		log.Printf("Aviso: não foi possível buscar indicador: %v", err)
		indicador = &models.Indicador{ID: req.IndicadorID, Nome: "Indicador não encontrado"}
	}

	// 6. Preparar dados para enviar ao Ollama (opcional, não afeta o status dos inserts)
	kpiWithIndicador := &models.KPI{
		ID:               uint(kpiID),
		Nome:             req.Nome,
		Descricao:        req.Descricao,
		TipoMeta:         req.TipoMeta,
		Periodicidade:    req.Periodicidade,
		FormulaCalculo:   req.FormulaCalculo,
		UnidadeMedidaKPI: req.UnidadeMedidaKPI,
		Indicador:        indicador,
	}

	// 7. Tentar gerar requisitos via Ollama (não bloqueia o retorno de sucesso)
	//var requirements *models.GeneratedRequirements
	go func() {
		// Chamada assíncrona para não bloquear a resposta
		log.Println("Iniciando geração assíncrona de requisitos via Ollama...")
		reqs, err := s.generateRequirementsFromKPI(kpiWithIndicador)
		if err != nil {
			log.Printf("Erro ao gerar requisitos via Ollama: %v", err)
			return
		}

		// Salvar requisitos em uma nova transação
		if err := s.saveGeneratedRequirements(uint(kpiID), reqs); err != nil {
			log.Printf("Erro ao salvar requisitos gerados: %v", err)
		} else {
			log.Printf("Requisitos gerados e salvos com sucesso para KPI %d", kpiID)
		}
	}()

	// 8. Montar resposta de sucesso
	response := &models.CompleteKPIResponse{
		KPI: &models.KPIResponse{
			ID:            uint(kpiID),
			Nome:          req.Nome,
			Descricao:     req.Descricao,
			IndicadorID:   req.IndicadorID,
			TipoMeta:      req.TipoMeta,
			Periodicidade: req.Periodicidade,
			CreatedAt:     time.Now(),
		},
		Metas:   convertToMetaResponses(metasCriadas),
		Success: true,
		Message: "KPI e metas criados com sucesso",
	}

	log.Printf("KPI %s criado com sucesso!", req.Nome)
	return response, nil
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

	// Chamar Ollama com timeout maior
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

// saveGeneratedRequirements salva os requisitos no banco (transação separada)
func (s *KPIService) saveGeneratedRequirements(kpiID uint, requirements *models.GeneratedRequirements) error {
	// Iniciar transação para salvar requisitos
	tx, err := s.db.Conn.Begin()
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %w", err)
	}
	defer tx.Rollback()

	// Salvar requisitos técnicos
	for _, req := range requirements.RequisitosTecnicos {
		_, err := tx.Exec(`
            INSERT INTO requisito_kpi 
            (kpi_id, tipo_requisito_id, codigo, titulo, descricao, 
             prioridade, complexidade_estimada, status, criado_por, data_criacao)
            VALUES (?, 1, ?, ?, ?, ?, ?, 'APROVADO', 'OLLAMA_GENERATOR', NOW())
        `, kpiID, req.Codigo, req.Titulo, req.Descricao, req.Prioridade, req.ComplexidadeEstimada)

		if err != nil {
			return fmt.Errorf("erro ao salvar requisito técnico %s: %w", req.Codigo, err)
		}
	}

	// Salvar requisitos não funcionais
	for _, req := range requirements.RequisitosNaoFuncionais {
		// Buscar ID da categoria
		var categoriaID *uint
		err := tx.QueryRow("SELECT id FROM categoria_nao_funcional WHERE nome = ?", req.Categoria).Scan(&categoriaID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("erro ao buscar categoria: %w", err)
		}

		_, err = tx.Exec(`
            INSERT INTO requisito_kpi 
            (kpi_id, tipo_requisito_id, codigo, titulo, descricao, 
             categoria_nao_funcional_id, prioridade, complexidade_estimada, 
             status, criado_por, data_criacao)
            VALUES (?, 2, ?, ?, ?, ?, ?, ?, 'APROVADO', 'OLLAMA_GENERATOR', NOW())
        `, kpiID, req.Codigo, req.Titulo, req.Descricao, categoriaID, req.Prioridade, req.ComplexidadeEstimada)

		if err != nil {
			return fmt.Errorf("erro ao salvar requisito não funcional %s: %w", req.Codigo, err)
		}
	}

	// Commit da transação de requisitos
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("erro ao commitar requisitos: %w", err)
	}

	return nil
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

// Funções auxiliares
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
