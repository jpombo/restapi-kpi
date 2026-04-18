package models

import "time"

// Estruturas para o banco de dados
type Indicador struct {
	ID            uint    `db:"id"`
	Nome          string  `db:"nome"`
	Descricao     *string `db:"descricao"`
	UnidadeMedida *string `db:"unidade_medida"`
}

type KPI struct {
	ID               uint       `db:"id"`
	IndicadorID      uint       `db:"indicador_id"`
	Nome             string     `db:"nome"`
	Descricao        *string    `db:"descricao"`
	TipoMeta         string     `db:"tipo_meta"`
	Periodicidade    string     `db:"periodicidade"`
	FormulaCalculo   *string    `db:"formula_calculo"`
	UnidadeMedidaKPI *string    `db:"unidade_medida_kpi"`
	Ativo            bool       `db:"ativo"`
	DataCriacao      time.Time  `db:"data_criacao"`
	Indicador        *Indicador `db:"-"`
}

// Estrutura para envio ao Ollama
type KPIRequest struct {
	ID               uint    `json:"id"`
	Nome             string  `json:"nome"`
	Descricao        *string `json:"descricao"`
	TipoMeta         string  `json:"tipo_meta"`
	Periodicidade    string  `json:"periodicidade"`
	FormulaCalculo   *string `json:"formula_calculo"`
	UnidadeMedidaKPI *string `json:"unidade_medida_kpi"`
	Indicador        struct {
		ID            uint    `json:"id"`
		Nome          string  `json:"nome"`
		UnidadeMedida *string `json:"unidade_medida"`
	} `json:"indicador"`
}

// Estrutura de resposta do Ollama
type OllamaResponse struct {
	Response string `json:"response"`
}

// Estrutura dos requisitos gerados
type GeneratedRequirements struct {
	KPIAnalisado struct {
		ID            uint   `json:"id"`
		Nome          string `json:"nome"`
		TipoMeta      string `json:"tipo_meta"`
		Periodicidade string `json:"periodicidade"`
	} `json:"kpi_analisado"`
	RequisitosTecnicos []struct {
		Codigo               string `json:"codigo"`
		Titulo               string `json:"titulo"`
		Descricao            string `json:"descricao"`
		Prioridade           string `json:"prioridade"`
		ComplexidadeEstimada string `json:"complexidade_estimada"`
	} `json:"requisitos_tecnicos"`
	RequisitosNaoFuncionais []struct {
		Codigo               string `json:"codigo"`
		Titulo               string `json:"titulo"`
		Descricao            string `json:"descricao"`
		Categoria            string `json:"categoria"`
		Prioridade           string `json:"prioridade"`
		ComplexidadeEstimada string `json:"complexidade_estimada"`
	} `json:"requisitos_nao_funcionais"`
	MetasSugeridas []struct {
		PeriodoReferencia string `json:"periodo_referencia"`
		TipoMeta          string `json:"tipo_meta"`
		Valor             string `json:"valor"`
		Observacao        string `json:"observacao"`
	} `json:"metas_sugeridas"`
	CriteriosAceitacao []struct {
		RequisitoCodigo string `json:"requisito_codigo"`
		Descricao       string `json:"descricao"`
		MetodoValidacao string `json:"metodo_validacao"`
	} `json:"criterios_aceitacao"`
}

type RequisitoKPI struct {
	ID                      uint      `db:"id"`
	KPIID                   uint      `db:"kpi_id"`
	TipoRequisitoID         uint      `db:"tipo_requisito_id"`
	Codigo                  string    `db:"codigo"`
	Titulo                  string    `db:"titulo"`
	Descricao               string    `db:"descricao"`
	CategoriaNaoFuncionalID *uint     `db:"categoria_nao_funcional_id"`
	Prioridade              string    `db:"prioridade"`
	ComplexidadeEstimada    string    `db:"complexidade_estimada"`
	Status                  string    `db:"status"`
	CriadoPor               string    `db:"criado_por"`
	DataCriacao             time.Time `db:"data_criacao"`
}

type Meta struct {
	ID                  uint     `db:"id"`
	KPIID               uint     `db:"kpi_id"`
	Ano                 uint     `db:"ano"`
	PeriodoReferencia   *string  `db:"periodo_referencia"`
	ValorMetaNumerica   *float64 `db:"valor_meta_numerica"`
	ValorMetaPercentual *float64 `db:"valor_meta_percentual"`
	ValorMetaMin        *float64 `db:"valor_meta_min"`
	ValorMetaMax        *float64 `db:"valor_meta_max"`
	TipoMetaRealizado   string   `db:"tipo_meta_realizado"`
	Observacao          *string  `db:"observacao"`
}
