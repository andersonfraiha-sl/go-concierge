package camadas

import (
	"context"
	stdsql "database/sql"
	"encoding/json"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/jackc/pgx/v5/pgconn"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/predicate"
	"go-concierge/internal/banco/ent/skillcamada"
	"go-concierge/internal/banco/ent/skillversao"
	"go-concierge/internal/falha"
)

const (
	timeoutConsulta         = 3 * time.Second
	timeoutTransacao        = 10 * time.Second
	timeoutRetrato          = 30 * time.Second
	tentativasSerializacao  = 3
	codigoFalhaSerializacao = "40001"
	maxLimite               = 500
	// maxCamadasPorSkill protege CamadasDaSkill: passar disso é erro, nunca corte silencioso
	// (a cascata de uma publicação esqueceria camadas).
	maxCamadasPorSkill = 5000
)

// Origem diz como uma versão nasceu.
type Origem string

const (
	OrigemImportacao  Origem = "importacao"
	OrigemInicial     Origem = "inicial"
	OrigemPublicacao  Origem = "publicacao"
	OrigemDecisao     Origem = "decisao"
	OrigemObrigatorio Origem = "obrigatorio"
	OrigemPrazo       Origem = "prazo"
	OrigemVolta       Origem = "volta"
)

// Versao é uma versão publicada de uma camada. AcimaID e AtualizacaoID valem 0 quando não há.
type Versao struct {
	ID, Numero, AcimaID, AtualizacaoID int
	Camada                             Camada
	Documento                          Documento
	CasoYAML                           []byte
	Origem                             Origem
	Autor, Resumo, Motivo              string
	Obrigatorios                       []TrechoID
	EncerraEm, CriadoEm                time.Time
}

// EstadoDaCamada é a camada como o editor a vê: a cabeça publicada, a versão de cima sobre a
// qual ela foi feita e o rascunho. Camada sem linha no banco volta só com Camada preenchida.
type EstadoDaCamada struct {
	Camada         Camada
	Cabeca         *Versao
	AcimaID        int
	Rascunho       *Documento
	VersaoRascunho string
}

// NovaVersao é o que InserirVersao grava. EncerraEm zero = a skill não tem fim marcado.
type NovaVersao struct {
	Camada                 Camada
	Documento              Documento
	CasoYAML               []byte
	AcimaID, AtualizacaoID int
	Origem                 Origem
	Autor, Resumo, Motivo  string
	Obrigatorios           []TrechoID
	EncerraEm              time.Time
	LimparRascunho         bool
}

// Postgres guarda camadas, versões, atualizações e avisos. Só o turno não passa por aqui: ele
// lê Efetivas, em memória.
type Postgres struct{ cli *ent.Client }

func NovoPostgres(cli *ent.Client) *Postgres { return &Postgres{cli: cli} }

// Repo lê e escreve fora de transação, uma operação por vez.
func (p *Postgres) Repo() Repo { return Repo{cli: p.cli} }

// EmTransacao roda f numa transação serializável e a refaz até tentativasSerializacao vezes
// quando o Postgres a aborta por conflito (40001), tudo dentro de timeoutTransacao. Erro de f
// desfaz tudo. No fim das tentativas devolve ErrConflitoConcorrente.
func (p *Postgres) EmTransacao(ctx context.Context, operacao string, f func(Repo) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeoutTransacao)
	defer cancel()
	for tentativa := 1; ; tentativa++ {
		err := p.tentar(ctx, f)
		if err == nil {
			return nil
		}
		if !falhaDeSerializacao(err) {
			return fmt.Errorf("%s: %w", operacao, err)
		}
		if tentativa == tentativasSerializacao || esperarNovaTentativa(ctx, tentativa) != nil {
			return fmt.Errorf("%s: %w após %d tentativas: %w", operacao, ErrConflitoConcorrente, tentativa, err)
		}
	}
}

func (p *Postgres) tentar(ctx context.Context, f func(Repo) error) error {
	tx, err := p.cli.BeginTx(ctx, &stdsql.TxOptions{Isolation: stdsql.LevelSerializable})
	if err != nil {
		return falha.ComPilha(fmt.Errorf("abrir transação: %w", err))
	}
	if err := f(Repo{cli: tx.Client()}); err != nil {
		if errDesfazer := tx.Rollback(); errDesfazer != nil {
			err = errors.Join(err, fmt.Errorf("desfazer transação: %w", errDesfazer))
		}
		return err
	}
	if err := tx.Commit(); err != nil {
		return falha.ComPilha(fmt.Errorf("confirmar transação: %w", err))
	}
	return nil
}

func falhaDeSerializacao(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	return ok && pgErr.Code == codigoFalhaSerializacao
}

// esperarNovaTentativa espaça as tentativas com um pouco de acaso, para os dois lados do
// conflito não colidirem de novo no mesmo instante.
func esperarNovaTentativa(ctx context.Context, tentativa int) error {
	espera := time.Duration(tentativa) * (10*time.Millisecond + rand.N(20*time.Millisecond))
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(espera):
		return nil
	}
}

// Repo é o acesso às tabelas: o cliente do banco, ou o da transação aberta por EmTransacao.
type Repo struct{ cli *ent.Client }

func comPrazo(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeoutConsulta)
}

func erroDeBanco(operacao string, err error) error {
	return falha.ComPilha(fmt.Errorf("%s: %w", operacao, err))
}

func limitar(n int) int { return min(max(n, 1), maxLimite) }

// TemBase diz se alguma skill já tem versão base publicada.
func (r Repo) TemBase(ctx context.Context) (bool, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	tem, err := r.cli.SkillCamada.Query().
		Where(skillcamada.TipoEQ(skillcamada.TipoBase), skillcamada.CabecaIDNotNil()).Exist(ctx)
	if err != nil {
		return false, erroDeBanco("conferir base publicada", err)
	}
	return tem, nil
}

// Camada devolve o estado da camada; sem linha, EstadoDaCamada{Camada: c}.
func (r Repo) Camada(ctx context.Context, c Camada) (EstadoDaCamada, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	quem, err := daCamada(c)
	if err != nil {
		return EstadoDaCamada{}, err
	}
	linha, err := r.cli.SkillCamada.Query().Where(quem).Only(ctx)
	if ent.IsNotFound(err) {
		return EstadoDaCamada{Camada: c}, nil
	}
	if err != nil {
		return EstadoDaCamada{}, erroDeBanco("ler camada "+c.Skill, err)
	}
	var cabeca *Versao
	if linha.CabecaID != nil {
		v, err := r.versao(ctx, *linha.CabecaID)
		if err != nil {
			return EstadoDaCamada{}, err
		}
		cabeca = &v
	}
	return estadoDe(linha, cabeca)
}

// CamadasDaSkill lista todas as camadas da skill (base, administradoras e condomínios) com a
// cabeça de cada uma, numa leitura só por tabela.
func (r Repo) CamadasDaSkill(ctx context.Context, skill string) ([]EstadoDaCamada, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	linhas, err := r.cli.SkillCamada.Query().Where(skillcamada.SkillEQ(skill)).
		Order(skillcamada.ByID()).Limit(maxCamadasPorSkill + 1).All(ctx)
	if err != nil {
		return nil, erroDeBanco("listar camadas de "+skill, err)
	}
	if len(linhas) > maxCamadasPorSkill {
		return nil, falha.ComPilha(fmt.Errorf("listar camadas de %s: mais de %d camadas", skill, maxCamadasPorSkill))
	}
	cabecas, err := r.cabecasDe(ctx, linhas)
	if err != nil {
		return nil, err
	}
	estados := make([]EstadoDaCamada, len(linhas))
	for i, linha := range linhas {
		estados[i], err = estadoDe(linha, cabecas[linha.ID])
		if err != nil {
			return nil, err
		}
	}
	return estados, nil
}

// cabecasDe lê as cabeças das camadas de uma vez, indexadas pelo id da camada.
func (r Repo) cabecasDe(ctx context.Context, linhas []*ent.SkillCamada) (map[int]*Versao, error) {
	donas := make(map[int]*ent.SkillCamada, len(linhas))
	var ids []int
	for _, l := range linhas {
		if l.CabecaID != nil {
			ids = append(ids, *l.CabecaID)
			donas[l.ID] = l
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	versoes, err := r.cli.SkillVersao.Query().Where(skillversao.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, erroDeBanco("ler cabeças das camadas", err)
	}
	cabecas := make(map[int]*Versao, len(versoes))
	for _, v := range versoes {
		v.Edges.Camada = donas[v.CamadaID]
		convertida, err := versaoDe(v)
		if err != nil {
			return nil, err
		}
		cabecas[v.CamadaID] = &convertida
	}
	return cabecas, nil
}

// SalvarRascunho grava o rascunho se versaoVista é a versão do rascunho atual ("" para quem
// nunca viu rascunho) e devolve a versão nova. A versão só cresce: publicar limpa o rascunho
// mas não zera o contador, para uma aba antiga nunca passar por cima de uma publicação.
func (r Repo) SalvarRascunho(ctx context.Context, c Camada, versaoVista string, d Documento) (string, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	quem, err := daCamada(c)
	if err != nil {
		return "", err
	}
	proxima, ok := proximaVersaoDoRascunho(versaoVista)
	if !ok {
		return "", ErrVersaoDesatualizada
	}
	bruto, err := json.Marshal(d)
	if err != nil {
		return "", fmt.Errorf("codificar rascunho de %s: %w", c.Skill, err)
	}
	n, err := r.cli.SkillCamada.Update().Where(quem, skillcamada.RascunhoVersaoEQ(versaoVista)).
		SetRascunho(jsontext.Value(bruto)).SetRascunhoVersao(proxima).Save(ctx)
	if err != nil {
		return "", erroDeBanco("salvar rascunho de "+c.Skill, err)
	}
	if n > 0 {
		return proxima, nil
	}
	existe, err := r.cli.SkillCamada.Query().Where(quem).Exist(ctx)
	if err != nil {
		return "", erroDeBanco("salvar rascunho de "+c.Skill, err)
	}
	if existe {
		return "", ErrVersaoDesatualizada
	}
	if _, err := r.novaLinha(c).SetRascunho(jsontext.Value(bruto)).SetRascunhoVersao(proxima).Save(ctx); err != nil {
		return "", erroDeBanco("criar camada de "+c.Skill, err)
	}
	return proxima, nil
}

// proximaVersaoDoRascunho: "" → "1", "n" → "n+1"; versão que não é número nunca foi emitida.
func proximaVersaoDoRascunho(vista string) (string, bool) {
	if vista == "" {
		return "1", true
	}
	n, err := strconv.Atoi(vista)
	if err != nil || n < 1 {
		return "", false
	}
	return strconv.Itoa(n + 1), true
}

// InserirVersao grava a versão com o número seguinte, cria a camada se falta e faz da nova a
// cabeça, com acima_id da camada igual a nv.AcimaID.
func (r Repo) InserirVersao(ctx context.Context, nv NovaVersao) (Versao, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	linha, err := r.camadaOuCria(ctx, nv.Camada)
	if err != nil {
		return Versao{}, err
	}
	numero, err := r.proximoNumero(ctx, linha.ID)
	if err != nil {
		return Versao{}, err
	}
	bruto, err := json.Marshal(nv.Documento)
	if err != nil {
		return Versao{}, fmt.Errorf("codificar versão de %s: %w", nv.Camada.Skill, err)
	}
	criar := r.cli.SkillVersao.Create().SetCamadaID(linha.ID).SetNumero(numero).
		SetDocumento(jsontext.Value(bruto)).SetOrigem(skillversao.Origem(nv.Origem)).
		SetAutor(nv.Autor).SetResumo(nv.Resumo).SetMotivo(nv.Motivo).
		SetObrigatorios(textos(nv.Obrigatorios))
	if len(nv.CasoYAML) > 0 {
		criar.SetCasoYaml(nv.CasoYAML)
	}
	if nv.AcimaID != 0 {
		criar.SetAcimaID(nv.AcimaID)
	}
	if nv.AtualizacaoID != 0 {
		criar.SetAtualizacaoID(nv.AtualizacaoID)
	}
	if !nv.EncerraEm.IsZero() {
		criar.SetEncerraEm(nv.EncerraEm)
	}
	salva, err := criar.Save(ctx)
	if err != nil {
		return Versao{}, erroDeBanco(fmt.Sprintf("gravar versão %d de %s", numero, nv.Camada.Skill), err)
	}
	if err := r.promoverCabeca(ctx, linha.ID, salva.ID, nv); err != nil {
		return Versao{}, err
	}
	salva.Edges.Camada = linha
	return versaoDe(salva)
}

func (r Repo) promoverCabeca(ctx context.Context, camadaID, versaoID int, nv NovaVersao) error {
	atualizar := r.cli.SkillCamada.UpdateOneID(camadaID).SetCabecaID(versaoID)
	if nv.AcimaID != 0 {
		atualizar.SetAcimaID(nv.AcimaID)
	} else {
		atualizar.ClearAcimaID()
	}
	if nv.LimparRascunho {
		atualizar.ClearRascunho()
	}
	if err := atualizar.Exec(ctx); err != nil {
		return erroDeBanco("mover cabeça de "+nv.Camada.Skill, err)
	}
	return nil
}

func (r Repo) proximoNumero(ctx context.Context, camadaID int) (int, error) {
	ultima, err := r.cli.SkillVersao.Query().Where(skillversao.CamadaID(camadaID)).
		Order(skillversao.ByNumero(sql.OrderDesc())).First(ctx)
	if ent.IsNotFound(err) {
		return 1, nil
	}
	if err != nil {
		return 0, erroDeBanco("numerar versão", err)
	}
	return ultima.Numero + 1, nil
}

// Versao lê uma versão pelo id.
func (r Repo) Versao(ctx context.Context, id int) (Versao, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	return r.versao(ctx, id)
}

func (r Repo) versao(ctx context.Context, id int) (Versao, error) {
	v, err := r.cli.SkillVersao.Query().Where(skillversao.ID(id)).WithCamada().Only(ctx)
	if ent.IsNotFound(err) {
		return Versao{}, fmt.Errorf("versão %d: %w", id, ErrVersaoNaoEncontrada)
	}
	if err != nil {
		return Versao{}, erroDeBanco(fmt.Sprintf("ler versão %d", id), err)
	}
	return versaoDe(v)
}

// VersaoPorNumero lê a versão de número dado na camada.
func (r Repo) VersaoPorNumero(ctx context.Context, c Camada, numero int) (Versao, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	quem, err := daCamada(c)
	if err != nil {
		return Versao{}, err
	}
	v, err := r.cli.SkillVersao.Query().
		Where(skillversao.HasCamadaWith(quem), skillversao.Numero(numero)).WithCamada().Only(ctx)
	if ent.IsNotFound(err) {
		return Versao{}, fmt.Errorf("versão %d de %s: %w", numero, c.Skill, ErrVersaoNaoEncontrada)
	}
	if err != nil {
		return Versao{}, erroDeBanco(fmt.Sprintf("ler versão %d de %s", numero, c.Skill), err)
	}
	return versaoDe(v)
}

// Historico lista as versões da camada, da mais nova para a mais antiga.
func (r Repo) Historico(ctx context.Context, c Camada, limite int) ([]Versao, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	quem, err := daCamada(c)
	if err != nil {
		return nil, err
	}
	linhas, err := r.cli.SkillVersao.Query().Where(skillversao.HasCamadaWith(quem)).
		Order(skillversao.ByNumero(sql.OrderDesc())).Limit(limitar(limite)).WithCamada().All(ctx)
	if err != nil {
		return nil, erroDeBanco("ler histórico de "+c.Skill, err)
	}
	return converterTodas(linhas, versaoDe)
}

// LerRetrato lê a cabeça de cada camada de uma vez, para a composição do turno. Cada cabeça é
// a vigente na hora da leitura; a camada criada no meio dela entra no próximo retrato.
func (r Repo) LerRetrato(ctx context.Context) (Retrato, error) {
	ctx, cancel := context.WithTimeout(ctx, timeoutRetrato)
	defer cancel()
	cabecas, err := r.cli.SkillVersao.Query().Where(ehCabeca()).WithCamada().All(ctx)
	if err != nil {
		return Retrato{}, erroDeBanco("ler retrato das camadas", err)
	}
	versoes, err := converterTodas(cabecas, versaoDe)
	if err != nil {
		return Retrato{}, err
	}
	return montarRetrato(versoes), nil
}

func ehCabeca() predicate.SkillVersao {
	return predicate.SkillVersao(func(s *sql.Selector) {
		cabecas := sql.Select(skillcamada.FieldCabecaID).From(sql.Table(skillcamada.Table)).
			Where(sql.NotNull(skillcamada.FieldCabecaID))
		s.Where(sql.In(s.C(skillversao.FieldID), cabecas))
	})
}

// montarRetrato é pura: distribui as cabeças por skill e por dono da camada.
func montarRetrato(cabecas []Versao) Retrato {
	r := Retrato{
		Bases:           map[string]VersaoBase{},
		Administradoras: map[string]map[string]Cabeca{},
		Condominios:     map[string]map[string]Cabeca{},
	}
	for _, v := range cabecas {
		cabeca := Cabeca{VersaoID: v.ID, Documento: v.Documento}
		c := v.Camada
		switch c.Tipo {
		case TipoBase:
			r.Bases[c.Skill] = VersaoBase{Cabeca: cabeca, CasoYAML: v.CasoYAML, EncerraEm: v.EncerraEm}
		case TipoAdministradora:
			guardarCabeca(r.Administradoras, c, cabeca)
		case TipoCondominio:
			guardarCabeca(r.Condominios, c, cabeca)
		}
	}
	return r
}

func guardarCabeca(destino map[string]map[string]Cabeca, c Camada, cabeca Cabeca) {
	if destino[c.Skill] == nil {
		destino[c.Skill] = map[string]Cabeca{}
	}
	destino[c.Skill][c.ID] = cabeca
}

// camadaOuCria devolve a linha da camada, criando-a se ainda não existe.
func (r Repo) camadaOuCria(ctx context.Context, c Camada) (*ent.SkillCamada, error) {
	quem, err := daCamada(c)
	if err != nil {
		return nil, err
	}
	linha, err := r.cli.SkillCamada.Query().Where(quem).Only(ctx)
	if ent.IsNotFound(err) {
		linha, err = r.novaLinha(c).Save(ctx)
	}
	if err != nil {
		return nil, erroDeBanco("obter camada de "+c.Skill, err)
	}
	return linha, nil
}

// idDaCamada é o id da linha de uma camada que já existe.
func (r Repo) idDaCamada(ctx context.Context, c Camada) (int, error) {
	quem, err := daCamada(c)
	if err != nil {
		return 0, err
	}
	id, err := r.cli.SkillCamada.Query().Where(quem).OnlyID(ctx)
	if ent.IsNotFound(err) {
		return 0, fmt.Errorf("camada de %s: %w", c.Skill, ErrCamadaNaoEncontrada)
	}
	if err != nil {
		return 0, erroDeBanco("obter camada de "+c.Skill, err)
	}
	return id, nil
}

func (r Repo) novaLinha(c Camada) *ent.SkillCamadaCreate {
	criar := r.cli.SkillCamada.Create().SetSkill(c.Skill).SetTipo(skillcamada.Tipo(c.Tipo))
	switch c.Tipo {
	case TipoAdministradora:
		criar.SetAdministradoraID(c.ID)
	case TipoCondominio:
		criar.SetCondominioID(c.ID)
	}
	return criar
}

// daCamada é o filtro que acha a camada; recusa a que não tem dono coerente com o tipo.
func daCamada(c Camada) (predicate.SkillCamada, error) {
	base := skillcamada.And(skillcamada.SkillEQ(c.Skill), skillcamada.TipoEQ(skillcamada.Tipo(c.Tipo)))
	switch {
	case c.Skill == "":
		return nil, fmt.Errorf("sem skill: %w", ErrCamadaInvalida)
	case c.Tipo == TipoBase && c.ID == "":
		return base, nil
	case c.Tipo == TipoAdministradora && c.ID != "":
		return skillcamada.And(base, skillcamada.AdministradoraIDEQ(c.ID)), nil
	case c.Tipo == TipoCondominio && c.ID != "":
		return skillcamada.And(base, skillcamada.CondominioIDEQ(c.ID)), nil
	}
	return nil, fmt.Errorf("%s %q de %s: %w", c.Tipo, c.ID, c.Skill, ErrCamadaInvalida)
}

func camadaDe(linha *ent.SkillCamada) Camada {
	c := Camada{Skill: linha.Skill, Tipo: Tipo(linha.Tipo)}
	switch {
	case linha.AdministradoraID != nil:
		c.ID = *linha.AdministradoraID
	case linha.CondominioID != nil:
		c.ID = *linha.CondominioID
	}
	return c
}

func estadoDe(linha *ent.SkillCamada, cabeca *Versao) (EstadoDaCamada, error) {
	estado := EstadoDaCamada{
		Camada: camadaDe(linha), Cabeca: cabeca,
		AcimaID: ouZero(linha.AcimaID), VersaoRascunho: linha.RascunhoVersao,
	}
	if len(linha.Rascunho) > 0 {
		var rascunho Documento
		if err := json.Unmarshal(linha.Rascunho, &rascunho); err != nil {
			return EstadoDaCamada{}, fmt.Errorf("rascunho de %s: %w", linha.Skill, err)
		}
		estado.Rascunho = &rascunho
	}
	return estado, nil
}

func versaoDe(v *ent.SkillVersao) (Versao, error) {
	if v.Edges.Camada == nil {
		return Versao{}, fmt.Errorf("versão %d sem camada carregada", v.ID)
	}
	var documento Documento
	if err := json.Unmarshal(v.Documento, &documento); err != nil {
		return Versao{}, fmt.Errorf("documento da versão %d: %w", v.ID, err)
	}
	obrigatorios := make([]TrechoID, len(v.Obrigatorios))
	for i, o := range v.Obrigatorios {
		obrigatorios[i] = TrechoID(o)
	}
	return Versao{
		ID: v.ID, Numero: v.Numero, AcimaID: ouZero(v.AcimaID), AtualizacaoID: ouZero(v.AtualizacaoID),
		Camada: camadaDe(v.Edges.Camada), Documento: documento, CasoYAML: v.CasoYaml,
		Origem: Origem(v.Origem), Autor: v.Autor, Resumo: v.Resumo, Motivo: v.Motivo,
		Obrigatorios: obrigatorios, EncerraEm: ouInstanteZero(v.EncerraEm), CriadoEm: v.CriadoEm,
	}, nil
}

func converterTodas[E, T any](linhas []E, converter func(E) (T, error)) ([]T, error) {
	convertidas := make([]T, len(linhas))
	for i, l := range linhas {
		c, err := converter(l)
		if err != nil {
			return nil, err
		}
		convertidas[i] = c
	}
	return convertidas, nil
}

func textos(ids []TrechoID) []string {
	t := make([]string, len(ids))
	for i, id := range ids {
		t[i] = string(id)
	}
	return t
}

func ouZero(n *int) int {
	if n == nil {
		return 0
	}
	return *n
}

func ouInstanteZero(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
