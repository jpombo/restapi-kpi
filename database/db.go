package database

import (
	"database/sql"
	"fmt"
	"log"

	"kpi-requirement-generator/config"
	"kpi-requirement-generator/models"

	_ "github.com/go-sql-driver/mysql"
)

type DB struct {
	Conn *sql.DB
}

func NewDB(cfg *config.Config) (*DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar ao banco: %w", err)
	}

	if err = conn.Ping(); err != nil {
		return nil, fmt.Errorf("erro ao pingar banco: %w", err)
	}

	log.Println("Conectado ao banco de dados com sucesso")

	return &DB{Conn: conn}, nil
}

func (db *DB) Close() error {
	return db.Conn.Close()
}

// CreateKPI insere um novo KPI e retorna o ID gerado
func (db *DB) CreateKPI(kpi *models.KPI) (uint, error) {
	query := `
        INSERT INTO kpi 
        (indicador_id, nome, descricao, tipo_meta, periodicidade, 
         formula_calculo, unidade_medida_kpi, ativo, data_criacao)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())
    `

	result, err := db.Conn.Exec(
		query,
		kpi.IndicadorID,
		kpi.Nome,
		kpi.Descricao,
		kpi.TipoMeta,
		kpi.Periodicidade,
		kpi.FormulaCalculo,
		kpi.UnidadeMedidaKPI,
		true,
	)

	if err != nil {
		return 0, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}

	return uint(id), nil
}

// CreateMeta insere uma nova meta
func (db *DB) CreateMeta(meta *models.Meta) error {
	query := `
        INSERT INTO meta 
        (kpi_id, ano, periodo_referencia, valor_meta_numerica, valor_meta_percentual,
         valor_meta_min, valor_meta_max, tipo_meta_realizado, observacao)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	_, err := db.Conn.Exec(
		query,
		meta.KPIID,
		meta.Ano,
		meta.PeriodoReferencia,
		meta.ValorMetaNumerica,
		meta.ValorMetaPercentual,
		meta.ValorMetaMin,
		meta.ValorMetaMax,
		meta.TipoMetaRealizado,
		meta.Observacao,
	)

	return err
}

// GetKPIByID busca um KPI completo com suas metas
func (db *DB) GetKPIByID(kpiID uint) (*models.KPI, []models.Meta, error) {
	// Buscar KPI
	kpiQuery := `
        SELECT id, indicador_id, nome, descricao, tipo_meta, periodicidade,
               formula_calculo, unidade_medida_kpi, ativo, data_criacao
        FROM kpi
        WHERE id = ? AND ativo = TRUE
    `

	var kpi models.KPI
	err := db.Conn.QueryRow(kpiQuery, kpiID).Scan(
		&kpi.ID, &kpi.IndicadorID, &kpi.Nome, &kpi.Descricao, &kpi.TipoMeta,
		&kpi.Periodicidade, &kpi.FormulaCalculo, &kpi.UnidadeMedidaKPI,
		&kpi.Ativo, &kpi.DataCriacao,
	)

	if err != nil {
		return nil, nil, err
	}

	// Buscar metas
	metasQuery := `
        SELECT id, kpi_id, ano, periodo_referencia, valor_meta_numerica,
               valor_meta_percentual, valor_meta_min, valor_meta_max,
               tipo_meta_realizado, observacao
        FROM meta
        WHERE kpi_id = ?
    `

	rows, err := db.Conn.Query(metasQuery, kpiID)
	if err != nil {
		return &kpi, nil, err
	}
	defer rows.Close()

	var metas []models.Meta
	for rows.Next() {
		var meta models.Meta
		err := rows.Scan(
			&meta.ID, &meta.KPIID, &meta.Ano, &meta.PeriodoReferencia,
			&meta.ValorMetaNumerica, &meta.ValorMetaPercentual,
			&meta.ValorMetaMin, &meta.ValorMetaMax,
			&meta.TipoMetaRealizado, &meta.Observacao,
		)
		if err != nil {
			return &kpi, metas, err
		}
		metas = append(metas, meta)
	}

	return &kpi, metas, nil
}

// GetIndicadorByID busca um indicador pelo ID
func (db *DB) GetIndicadorByID(indicadorID uint) (*models.Indicador, error) {
	query := `SELECT id, nome, descricao, unidade_medida FROM indicador WHERE id = ?`

	var indicador models.Indicador
	err := db.Conn.QueryRow(query, indicadorID).Scan(
		&indicador.ID, &indicador.Nome, &indicador.Descricao, &indicador.UnidadeMedida,
	)

	if err != nil {
		return nil, err
	}

	return &indicador, nil
}

// GetKPIWithIndicador busca um KPI pelo ID com seus dados do indicador
func (db *DB) GetKPIWithIndicador(kpiID uint) (*models.KPI, error) {
	query := `
        SELECT 
            k.id, k.indicador_id, k.nome, k.descricao, k.tipo_meta, 
            k.periodicidade, k.formula_calculo, k.unidade_medida_kpi, 
            k.ativo, k.data_criacao,
            i.id AS indicador_id, i.nome AS indicador_nome, 
            i.descricao AS indicador_descricao, i.unidade_medida AS indicador_unidade
        FROM kpi k
        JOIN indicador i ON k.indicador_id = i.id
        WHERE k.id = ? AND k.ativo = TRUE
    `

	row := db.Conn.QueryRow(query, kpiID)

	var kpi models.KPI
	var indicador models.Indicador

	err := row.Scan(
		&kpi.ID, &kpi.IndicadorID, &kpi.Nome, &kpi.Descricao, &kpi.TipoMeta,
		&kpi.Periodicidade, &kpi.FormulaCalculo, &kpi.UnidadeMedidaKPI,
		&kpi.Ativo, &kpi.DataCriacao,
		&indicador.ID, &indicador.Nome, &indicador.Descricao, &indicador.UnidadeMedida,
	)

	if err != nil {
		return nil, err
	}

	kpi.Indicador = &indicador
	return &kpi, nil
}

// GetCategoriaNaoFuncionalID busca o ID da categoria pelo nome
func (db *DB) GetCategoriaNaoFuncionalID(nome string) (*uint, error) {
	query := `SELECT id FROM categoria_nao_funcional WHERE nome = ?`
	var id uint
	err := db.Conn.QueryRow(query, nome).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &id, nil
}

// SaveRequisito salva um requisito no banco
func (db *DB) SaveRequisito(req *models.RequisitoKPI) error {
	query := `
        INSERT INTO requisito_kpi 
        (kpi_id, tipo_requisito_id, codigo, titulo, descricao, 
         categoria_nao_funcional_id, prioridade, complexidade_estimada, 
         status, criado_por)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	_, err := db.Conn.Exec(
		query,
		req.KPIID,
		req.TipoRequisitoID,
		req.Codigo,
		req.Titulo,
		req.Descricao,
		req.CategoriaNaoFuncionalID,
		req.Prioridade,
		req.ComplexidadeEstimada,
		req.Status,
		req.CriadoPor,
	)

	return err
}

// SaveMeta salva uma meta no banco
func (db *DB) SaveMeta(meta *models.Meta) error {
	query := `
        INSERT INTO meta 
        (kpi_id, ano, periodo_referencia, valor_meta_numerica, valor_meta_percentual,
         valor_meta_min, valor_meta_max, tipo_meta_realizado, observacao)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    `

	_, err := db.Conn.Exec(
		query,
		meta.KPIID,
		meta.Ano,
		meta.PeriodoReferencia,
		meta.ValorMetaNumerica,
		meta.ValorMetaPercentual,
		meta.ValorMetaMin,
		meta.ValorMetaMax,
		meta.TipoMetaRealizado,
		meta.Observacao,
	)

	return err
}
