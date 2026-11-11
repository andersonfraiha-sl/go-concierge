package camadas

import (
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"

	"go-concierge/internal/banco/ent"
	"go-concierge/internal/banco/ent/skillatualizacao"
	"go-concierge/internal/banco/ent/skillaviso"
)

// EstadoAtualizacao é onde uma atualização está: aberta (pendente, recusada, parcial) ou
// encerrada (aceita, substituida).
type EstadoAtualizacao string

const (
	EstadoPendente    EstadoAtualizacao = "pendente"
	EstadoAceita      EstadoAtualizacao = "aceita"
	EstadoRecusada    EstadoAtualizacao = "recusada"
	EstadoParcial     EstadoAtualizacao = "parcial"
	EstadoSubstituida EstadoAtualizacao = "substituida"
)

var (
	estadosAbertos   = []skillatualizacao.Estado{skillatualizacao.EstadoPendente, skillatualizacao.EstadoRecusada, skillatualizacao.EstadoParcial}
	estadosVenciveis = []skillatualizacao.Estado{skillatualizacao.EstadoPendente, skillatualizacao.EstadoParcial}
)

// Atualizacao é a versão de cima que passou de DeID para ParaID e que a camada ainda decide.
type Atualizacao struct {
	ID            int
	Camada        Camada
	DeID, ParaID  int
	Estado        EstadoAtualizacao
	PrazoEm       time.Time
	PrazoAplicado bool
	Decisoes      []Decisao
}

// NovoAviso avisa quem tinha personalização num trecho trocado por versão obrigatória, ou numa
// skill que a base encerrou. EncerraEm zero = sem data de fim.
type NovoAviso struct {
	Camada    Camada
	VersaoID  int
	Motivo    string
	Trechos   []string
	EncerraEm time.Time
}

// Aviso é um NovoAviso gravado, ainda sem ciente.
type Aviso struct {
	ID int
	NovoAviso
	CriadoEm time.Time
}

// CriarAtualizacao abre a atualização da camada e marca como substituída a que ainda estava
// aberta: só a mais nova pede decisão.
func (r Repo) CriarAtualizacao(ctx context.Context, c Camada, deID, paraID int, prazoEm time.Time) (int, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	camadaID, err := r.idDaCamada(ctx, c)
	if err != nil {
		return 0, err
	}
	_, err = r.cli.SkillAtualizacao.Update().
		Where(skillatualizacao.CamadaID(camadaID), skillatualizacao.EstadoIn(estadosAbertos...)).
		SetEstado(skillatualizacao.EstadoSubstituida).Save(ctx)
	if err != nil {
		return 0, erroDeBanco("substituir atualizações de "+c.Skill, err)
	}
	nova, err := r.cli.SkillAtualizacao.Create().SetCamadaID(camadaID).
		SetDeID(deID).SetParaID(paraID).SetPrazoEm(prazoEm).Save(ctx)
	if err != nil {
		return 0, erroDeBanco("criar atualização de "+c.Skill, err)
	}
	return nova.ID, nil
}

// Atualizacao lê uma atualização pelo id.
func (r Repo) Atualizacao(ctx context.Context, id int) (Atualizacao, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	a, err := r.cli.SkillAtualizacao.Query().Where(skillatualizacao.ID(id)).WithCamada().Only(ctx)
	if ent.IsNotFound(err) {
		return Atualizacao{}, fmt.Errorf("atualização %d: %w", id, ErrAtualizacaoNaoEncontrada)
	}
	if err != nil {
		return Atualizacao{}, erroDeBanco(fmt.Sprintf("ler atualização %d", id), err)
	}
	return atualizacaoDe(a)
}

// AtualizacoesAbertas lista as que ainda esperam decisão (pendente, recusada, parcial), da
// mais nova para a mais antiga.
func (r Repo) AtualizacoesAbertas(ctx context.Context, limite int) ([]Atualizacao, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	linhas, err := r.cli.SkillAtualizacao.Query().Where(skillatualizacao.EstadoIn(estadosAbertos...)).
		Order(skillatualizacao.ByID(sql.OrderDesc())).Limit(limitar(limite)).WithCamada().All(ctx)
	if err != nil {
		return nil, erroDeBanco("listar atualizações abertas", err)
	}
	return converterTodas(linhas, atualizacaoDe)
}

// AtualizacoesVencidas lista as pendentes ou parciais cujo prazo chegou e que o sistema ainda
// não aplicou, da mais atrasada para a menos.
func (r Repo) AtualizacoesVencidas(ctx context.Context, agora time.Time, limite int) ([]Atualizacao, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	linhas, err := r.cli.SkillAtualizacao.Query().
		Where(skillatualizacao.EstadoIn(estadosVenciveis...), skillatualizacao.PrazoEmLTE(agora), skillatualizacao.PrazoAplicado(false)).
		Order(skillatualizacao.ByPrazoEm(), skillatualizacao.ByID()).Limit(limitar(limite)).WithCamada().All(ctx)
	if err != nil {
		return nil, erroDeBanco("listar atualizações vencidas", err)
	}
	return converterTodas(linhas, atualizacaoDe)
}

// RegistrarDecisao grava o estado novo da atualização, as decisões e quem decidiu.
func (r Repo) RegistrarDecisao(ctx context.Context, id int, e EstadoAtualizacao, ds []Decisao, por string, em time.Time) error {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	bruto, err := json.Marshal(ds)
	if err != nil {
		return fmt.Errorf("codificar decisões da atualização %d: %w", id, err)
	}
	err = r.cli.SkillAtualizacao.UpdateOneID(id).SetEstado(skillatualizacao.Estado(e)).
		SetDecisoes(jsontext.Value(bruto)).SetDecididaPor(por).SetDecididaEm(em).Exec(ctx)
	return erroDeAtualizacao(id, "registrar decisão da atualização", err)
}

// MarcarPrazoAplicado registra que o sistema já aplicou o prazo desta atualização.
func (r Repo) MarcarPrazoAplicado(ctx context.Context, id int) error {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	err := r.cli.SkillAtualizacao.UpdateOneID(id).SetPrazoAplicado(true).Exec(ctx)
	return erroDeAtualizacao(id, "marcar prazo aplicado da atualização", err)
}

func erroDeAtualizacao(id int, operacao string, err error) error {
	if err == nil {
		return nil
	}
	if ent.IsNotFound(err) {
		return fmt.Errorf("atualização %d: %w", id, ErrAtualizacaoNaoEncontrada)
	}
	return erroDeBanco(fmt.Sprintf("%s %d", operacao, id), err)
}

// CriarAviso grava um aviso para a camada.
func (r Repo) CriarAviso(ctx context.Context, a NovoAviso) error {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	camadaID, err := r.idDaCamada(ctx, a.Camada)
	if err != nil {
		return err
	}
	trechos := a.Trechos
	if trechos == nil {
		trechos = []string{}
	}
	criar := r.cli.SkillAviso.Create().SetCamadaID(camadaID).SetVersaoID(a.VersaoID).
		SetMotivo(a.Motivo).SetTrechos(trechos)
	if !a.EncerraEm.IsZero() {
		criar.SetEncerraEm(a.EncerraEm)
	}
	if err := criar.Exec(ctx); err != nil {
		return erroDeBanco("criar aviso de "+a.Camada.Skill, err)
	}
	return nil
}

// AvisosAbertos lista os avisos sem ciente, do mais novo para o mais antigo.
func (r Repo) AvisosAbertos(ctx context.Context, limite int) ([]Aviso, error) {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	linhas, err := r.cli.SkillAviso.Query().Where(skillaviso.CienteEmIsNil()).
		Order(skillaviso.ByID(sql.OrderDesc())).Limit(limitar(limite)).WithCamada().All(ctx)
	if err != nil {
		return nil, erroDeBanco("listar avisos abertos", err)
	}
	return converterTodas(linhas, avisoDe)
}

// MarcarCiente registra o ciente; quem dá ciente de novo não troca o primeiro registro.
func (r Repo) MarcarCiente(ctx context.Context, id int, por string, em time.Time) error {
	ctx, cancel := comPrazo(ctx)
	defer cancel()
	n, err := r.cli.SkillAviso.Update().Where(skillaviso.ID(id), skillaviso.CienteEmIsNil()).
		SetCientePor(por).SetCienteEm(em).Save(ctx)
	if err != nil {
		return erroDeBanco(fmt.Sprintf("dar ciente do aviso %d", id), err)
	}
	if n > 0 {
		return nil
	}
	existe, err := r.cli.SkillAviso.Query().Where(skillaviso.ID(id)).Exist(ctx)
	if err != nil {
		return erroDeBanco(fmt.Sprintf("dar ciente do aviso %d", id), err)
	}
	if !existe {
		return fmt.Errorf("aviso %d: %w", id, ErrAvisoNaoEncontrado)
	}
	return nil
}

func atualizacaoDe(a *ent.SkillAtualizacao) (Atualizacao, error) {
	if a.Edges.Camada == nil {
		return Atualizacao{}, fmt.Errorf("atualização %d sem camada carregada", a.ID)
	}
	var decisoes []Decisao
	if len(a.Decisoes) > 0 {
		if err := json.Unmarshal(a.Decisoes, &decisoes); err != nil {
			return Atualizacao{}, fmt.Errorf("decisões da atualização %d: %w", a.ID, err)
		}
	}
	return Atualizacao{
		ID: a.ID, Camada: camadaDe(a.Edges.Camada), DeID: a.DeID, ParaID: a.ParaID,
		Estado: EstadoAtualizacao(a.Estado), PrazoEm: a.PrazoEm, PrazoAplicado: a.PrazoAplicado,
		Decisoes: decisoes,
	}, nil
}

func avisoDe(a *ent.SkillAviso) (Aviso, error) {
	if a.Edges.Camada == nil {
		return Aviso{}, fmt.Errorf("aviso %d sem camada carregada", a.ID)
	}
	return Aviso{
		ID: a.ID, CriadoEm: a.CriadoEm,
		NovoAviso: NovoAviso{
			Camada: camadaDe(a.Edges.Camada), VersaoID: a.VersaoID, Motivo: a.Motivo,
			Trechos: a.Trechos, EncerraEm: ouInstanteZero(a.EncerraEm),
		},
	}, nil
}
