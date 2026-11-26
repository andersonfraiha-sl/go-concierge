package catalogo

import (
	"context"
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"gopkg.in/yaml.v3"

	"go-concierge/internal/core/banco/ent"
	entcatalogo "go-concierge/internal/core/banco/ent/catalogoarquivo"
	"go-concierge/internal/core/banco/ent/predicate"
	"go-concierge/internal/falha"
	"go-concierge/internal/harness/persistencia"
)

const (
	// teto de cada ida ao banco; o chamador pode apertar com o próprio contexto.
	timeoutCatalogo = 30 * time.Second
	// o upsert vai em lotes para o pedido não crescer com o catálogo inteiro.
	arquivosPorLote = 100
)

// Postgres guarda o catálogo na tabela catalog_files.
type Postgres struct{ cli *ent.Client }

// NovoPostgres usa cli, que o chamador abre e fecha.
func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

// Mudanca conta o que Substituir levou ao banco.
type Mudanca struct{ Gravados, Removidos int }

// Listar devolve todos os arquivos do catálogo, ordenados por caminho.
func (p *Postgres) Listar(ctx context.Context) ([]Arquivo, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	linhas, err := p.cli.CatalogoArquivo.Query().Order(ent.Asc(entcatalogo.FieldID)).All(ctx)
	if err != nil {
		return nil, falha.ComPilha(fmt.Errorf("listar catálogo: %w", err))
	}
	out := make([]Arquivo, len(linhas))
	for i, l := range linhas {
		out[i] = Arquivo{Caminho: l.ID, Conteudo: l.Conteudo}
	}
	return out, nil
}

func (p *Postgres) ListarPagina(ctx context.Context, caminho string, deslocamento int64) (Pagina, error) {
	if deslocamento < 0 {
		return Pagina{}, fmt.Errorf("deslocamento inválido")
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	rows, err := p.cli.QueryContext(ctx, `WITH versao AS (
		SELECT md5(COALESCE(string_agg(length(path)::text || ':' || path || sha256, '' ORDER BY path COLLATE "C"), '')) AS valor FROM catalog_files
	), fragmentos AS (
		SELECT path, octet_length(content)::bigint AS total,
		CASE WHEN path COLLATE "C" = $1::text COLLATE "C" THEN $2::bigint ELSE 0 END AS inicio,
		substring(content FROM (CASE WHEN path COLLATE "C" = $1::text COLLATE "C" THEN $2 ELSE 0 END)::int + 1 FOR $3) AS conteudo
		FROM catalog_files WHERE path COLLATE "C" > $1::text COLLATE "C" OR (path COLLATE "C" = $1::text COLLATE "C" AND $2 < octet_length(content))
		ORDER BY path COLLATE "C" LIMIT $4
	) SELECT versao.valor, fragmentos.path, fragmentos.total, fragmentos.inicio, fragmentos.conteudo
	FROM versao LEFT JOIN fragmentos ON true ORDER BY fragmentos.path COLLATE "C"`, caminho, deslocamento, TamanhoTrecho, ArquivosPorPagina)
	if err != nil {
		return Pagina{}, falha.ComPilha(fmt.Errorf("listar página do catálogo: %w", err))
	}
	defer rows.Close()
	out := Pagina{Trechos: []Trecho{}, Proximo: caminho, Deslocamento: deslocamento, Fim: true}
	for rows.Next() {
		var versao string
		var nome *string
		var total, inicio *int64
		var conteudo []byte
		if err := rows.Scan(&versao, &nome, &total, &inicio, &conteudo); err != nil {
			return Pagina{}, err
		}
		out.Versao = versao
		if nome == nil {
			continue
		}
		t := Trecho{Caminho: *nome, Total: *total, Inicio: *inicio, Conteudo: conteudo}
		out.Trechos = append(out.Trechos, t)
		out.Proximo, out.Deslocamento = t.Caminho, t.Inicio+int64(len(t.Conteudo))
		if out.Deslocamento < t.Total || len(out.Trechos) == ArquivosPorPagina {
			out.Fim = false
			break
		}
	}
	if err := rows.Err(); err != nil {
		return Pagina{}, err
	}
	return out, nil
}

func (p *Postgres) ListarParaExecucao(ctx context.Context, agente, conta string) ([]Arquivo, error) {
	selecao, err := SelecionarExecucao(agente, conta)
	if err != nil {
		return nil, err
	}
	porPrefixo := func(prefixo string) predicate.CatalogoArquivo {
		return func(s *entsql.Selector) { s.Where(entsql.HasPrefix(s.C(entcatalogo.FieldID), prefixo)) }
	}
	filtros := []predicate.CatalogoArquivo{porPrefixo(selecao.Raiz + "/")}
	for _, ancestral := range selecao.Ancestrais {
		filtros = append(filtros, entcatalogo.And(porPrefixo(ancestral+"/"), entcatalogo.Not(porPrefixo(ancestral+"/agentes/"))))
	}
	if selecao.Fixture != "" {
		filtros = append(filtros, entcatalogo.IDEQ(selecao.Fixture))
	}
	ctx, cancelar := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancelar()
	linhas, err := p.cli.CatalogoArquivo.Query().Where(entcatalogo.Or(filtros...)).Order(ent.Asc(entcatalogo.FieldID)).All(ctx)
	if err != nil {
		return nil, falha.ComPilha(fmt.Errorf("listar catálogo da execução: %w", err))
	}
	arquivos := make([]Arquivo, len(linhas))
	for i, linha := range linhas {
		arquivos[i] = Arquivo{Caminho: linha.ID, Conteudo: linha.Conteudo}
	}
	return arquivos, nil
}

// Resumos devolve caminho → sha256 de tudo que está no banco, sem trazer os bytes.
func (p *Postgres) Resumos(ctx context.Context) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	return resumos(ctx, p.cli.CatalogoArquivo)
}

// Vazio diz se a tabela não tem nenhuma linha.
func (p *Postgres) Vazio(ctx context.Context) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	existe, err := p.cli.CatalogoArquivo.Query().Exist(ctx)
	if err != nil {
		return false, falha.ComPilha(fmt.Errorf("conferir se o catálogo está vazio: %w", err))
	}
	return !existe, nil
}

func (p *Postgres) InserirCatalogoNexusSeAusente(ctx context.Context, arquivo Arquivo) error {
	if err := ValidarCaminho(arquivo.Caminho); err != nil {
		return err
	}
	if !strings.HasPrefix(arquivo.Caminho, "nexus/catalogo-nexus/") {
		return fmt.Errorf("caminho não é catálogo Nexus editável: %q", arquivo.Caminho)
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	_, err := p.cli.ExecContext(ctx, `INSERT INTO catalog_files(path,content,sha256,updated_at) VALUES($1,$2,$3,$4) ON CONFLICT(path) DO NOTHING`,
		arquivo.Caminho, arquivo.Conteudo, SHA256(arquivo.Conteudo), time.Now().UTC())
	if err != nil {
		return errors.Join(persistencia.ErrGravacaoIncerta, fmt.Errorf("importar catálogo Nexus: %w", err))
	}
	return nil
}

func (p *Postgres) Substituir(ctx context.Context, noDisco []Arquivo, remover ...string) (Mudanca, error) {
	if _, err := projecoesDe(noDisco); err != nil {
		return Mudanca{}, err
	}
	for _, a := range noDisco {
		if err := ValidarCaminho(a.Caminho); err != nil {
			return Mudanca{}, fmt.Errorf("substituir catálogo: %w", err)
		}
	}
	alvos := make(map[string]bool, len(noDisco))
	for _, a := range noDisco {
		alvos[a.Caminho] = true
	}
	for _, caminho := range remover {
		if err := ValidarCaminho(caminho); err != nil {
			return Mudanca{}, err
		}
		if alvos[caminho] {
			return Mudanca{}, fmt.Errorf("caminho %q presente na gravação e na remoção", caminho)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutCatalogo)
	defer cancel()
	tx, err := p.cli.Tx(ctx)
	if err != nil {
		return Mudanca{}, falha.ComPilha(fmt.Errorf("substituir catálogo: abrir transação: %w", err))
	}
	mudanca, err := aplicar(ctx, tx, noDisco, remover)
	if err != nil {
		if errDesfazer := tx.Rollback(); errDesfazer != nil {
			err = errors.Join(err, fmt.Errorf("desfazer transação: %w", errDesfazer))
		}
		return Mudanca{}, falha.ComPilha(fmt.Errorf("substituir catálogo: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return Mudanca{}, errors.Join(persistencia.ErrGravacaoIncerta, falha.ComPilha(fmt.Errorf("substituir catálogo: confirmar transação: %w", err)))
	}
	return mudanca, nil
}

func aplicar(ctx context.Context, tx *ent.Tx, noDisco []Arquivo, remover []string) (Mudanca, error) {
	atuais, err := resumos(ctx, tx.CatalogoArquivo)
	if err != nil {
		return Mudanca{}, err
	}
	gravar, _ := Diferenca(atuais, noDisco)
	agora := time.Now().UTC()
	for lote := range slices.Chunk(gravar, arquivosPorLote) {
		if err := gravarLote(ctx, tx, lote, agora); err != nil {
			return Mudanca{}, err
		}
	}
	if err := projetarCasos(ctx, tx.Client(), noDisco, agora); err != nil {
		return Mudanca{}, err
	}
	removidos := 0
	if len(remover) > 0 {
		for _, caminho := range remover {
			if err := removerCasoSemUso(ctx, tx.Client(), caminho); err != nil {
				return Mudanca{}, err
			}
		}
		var err error
		if removidos, err = tx.CatalogoArquivo.Delete().Where(entcatalogo.IDIn(remover...)).Exec(ctx); err != nil {
			return Mudanca{}, fmt.Errorf("remover %d arquivo(s): %w", len(remover), err)
		}
	}
	return Mudanca{Gravados: len(gravar), Removidos: removidos}, nil
}

func gravarLote(ctx context.Context, tx *ent.Tx, lote []Arquivo, agora time.Time) error {
	criar := make([]*ent.CatalogoArquivoCreate, len(lote))
	for i, a := range lote {
		criar[i] = tx.CatalogoArquivo.Create().SetID(a.Caminho).SetConteudo(a.Conteudo).
			SetSha256(SHA256(a.Conteudo)).SetAtualizadoEm(agora)
	}
	err := tx.CatalogoArquivo.CreateBulk(criar...).OnConflictColumns(entcatalogo.FieldID).UpdateNewValues().Exec(ctx)
	if err != nil {
		return fmt.Errorf("gravar %d arquivo(s): %w", len(lote), err)
	}
	return nil
}

func resumos(ctx context.Context, cli *ent.CatalogoArquivoClient) (map[string]string, error) {
	var linhas []struct {
		ID     string `json:"path"`
		Sha256 string `json:"sha256"`
	}
	if err := cli.Query().Select(entcatalogo.FieldID, entcatalogo.FieldSha256).Scan(ctx, &linhas); err != nil {
		return nil, falha.ComPilha(fmt.Errorf("ler resumos do catálogo: %w", err))
	}
	out := make(map[string]string, len(linhas))
	for _, l := range linhas {
		out[l.ID] = l.Sha256
	}
	return out, nil
}

type casoProjetado struct {
	ID               string           `yaml:"id"`
	Name             string           `yaml:"name"`
	Summary          string           `yaml:"summary"`
	Icon             string           `yaml:"icon"`
	Visibility       string           `yaml:"visibility"`
	Steps            []map[string]any `yaml:"steps"`
	file, agent, sha string
}

func projecoesDe(arquivos []Arquivo) (map[string]casoProjetado, error) {
	casos := map[string]casoProjetado{}
	for _, a := range arquivos {
		if path.Base(a.Caminho) != "caso.yaml" || !strings.HasPrefix(a.Caminho, "agentes/") {
			continue
		}
		var c casoProjetado
		if err := yaml.Unmarshal(a.Conteudo, &c); err != nil {
			return nil, fmt.Errorf("%s: caso inválido: %w", a.Caminho, err)
		}
		if strings.TrimSpace(c.ID) == "" || len(c.Steps) == 0 {
			return nil, fmt.Errorf("%s: caso exige ID global e passos", a.Caminho)
		}
		if anterior, existe := casos[c.ID]; existe {
			return nil, fmt.Errorf("ID global %q repetido em %s e %s", c.ID, anterior.file, a.Caminho)
		}
		c.file, c.agent, c.sha = a.Caminho, path.Dir(a.Caminho), SHA256(a.Conteudo)
		if c.Visibility == "" {
			c.Visibility = "private"
		}
		casos[c.ID] = c
	}
	return casos, nil
}

func projetarCasos(ctx context.Context, cli *ent.Client, arquivos []Arquivo, agora time.Time) error {
	casos, err := projecoesDe(arquivos)
	if err != nil {
		return err
	}
	for _, c := range casos {
		result, err := cli.ExecContext(ctx, `INSERT INTO use_cases(id,file_path,agent_path,name,summary,icon,visibility,source_sha256,updated_at) VALUES($1,$2,$3,$4,$5,NULLIF($6,''),$7,$8,$9) ON CONFLICT(id) DO UPDATE SET file_path=EXCLUDED.file_path,agent_path=EXCLUDED.agent_path,name=EXCLUDED.name,summary=EXCLUDED.summary,icon=EXCLUDED.icon,visibility=EXCLUDED.visibility,source_sha256=EXCLUDED.source_sha256,updated_at=EXCLUDED.updated_at WHERE use_cases.file_path=EXCLUDED.file_path`, c.ID, c.file, c.agent, c.Name, c.Summary, c.Icon, c.Visibility, c.sha, agora)
		if err != nil {
			return fmt.Errorf("projetar caso %q: %w", c.ID, err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("ID global %q já pertence a outro arquivo", c.ID)
		}
	}
	return nil
}

func removerCasoSemUso(ctx context.Context, cli *ent.Client, caminho string) error {
	rows, err := cli.QueryContext(ctx, `SELECT u.id, EXISTS(SELECT 1 FROM company_use_cases c WHERE c.use_case_id=u.id) FROM use_cases u WHERE u.file_path=$1 FOR UPDATE`, caminho)
	if err != nil {
		return err
	}
	var id string
	if !rows.Next() {
		err := rows.Err()
		rows.Close()
		return err
	}
	var emUso bool
	err = rows.Scan(&id, &emUso)
	rows.Close()
	if err != nil {
		return err
	}
	if emUso {
		return fmt.Errorf("caso %q configurado por empresa; remoção recusada", id)
	}
	result, err := cli.ExecContext(ctx, `DELETE FROM use_cases u WHERE u.id=$1 AND u.file_path=$2 AND NOT EXISTS(SELECT 1 FROM company_use_cases c WHERE c.use_case_id=u.id)`, id, caminho)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("caso %q alterado durante remoção", id)
	}
	return nil
}
