package atende

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"go.opentelemetry.io/otel/metric/noop"

	"go-concierge/internal/admin"
	"go-concierge/internal/entrada"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/pii"
	"go-concierge/internal/memoria"
)

// MotivoFalhaDeGancho é o motivo do transbordo quando um fixo de mensagem_recebida
// falha: a mensagem não pode seguir sem a checagem e a máscara (spec §11).
const MotivoFalhaDeGancho = "não consegui atender agora"

// ganchos é o conjunto injetado ou, sem ele, o dos fixos, montado no primeiro uso.
func (s *Servico) ganchos() *ganchos.Conjunto {
	if s.Ganchos != nil {
		return s.Ganchos
	}
	s.fixosUmaVez.Do(func() { s.soFixos = conjuntoDosFixos() })
	return s.soFixos
}

// conjuntoDosFixos é o conjunto de quem não injetou um: só fixos, com log e métrica noop,
// para falha de fixo sair em ERROR (o mesmo que harness.Build monta).
func conjuntoDosFixos() *ganchos.Conjunto {
	observar, err := ganchos.ComObservabilidade(slog.Default(), noop.NewMeterProvider().Meter("ganchos"))
	if err != nil {
		slog.Error("atende: ganchos sem observabilidade", "operacao", "montar_ganchos", "erro", err)
		return ganchos.NovoConjunto(fixos.Todos(), nil)
	}
	return ganchos.NovoConjunto(fixos.Todos(), nil, observar)
}

// lerMapeamento é o mapeamento que mensagem_recebida resolve. Sem repositório nenhum
// condomínio é conhecido: o mapeamento vazio faz o escopo_da_conversa bloquear.
func (s *Servico) lerMapeamento(ctx context.Context) (admin.Mapeamento, error) {
	if s.Mapeamento == nil {
		return admin.Mapeamento{}, nil
	}
	return s.Mapeamento.Ler(ctx)
}

// mensagemRecebida passa a mensagem pelos ganchos: o escopo resolve o condomínio e o
// texto sai mascarado, com os valores no cofre do turno.
func (s *Servico) mensagemRecebida(ctx context.Context, sess *Sessao, t ganchos.Turno, msg entrada.Mensagem, texto string, m admin.Mapeamento) ganchos.Resultado[ganchos.MensagemRecebida] {
	r := t.MensagemRecebida(ctx, ganchos.MensagemRecebida{
		Agora: time.Now(), Texto: texto, CompanyID: msg.CompanyID, Tenant: msg.Tenant,
		Mapeamento: m, Implantacao: s.Implantacao, ContatoDaSessao: sess.contato(),
		Identidade: sess.identidadeDosGanchos(),
	})
	sess.anotarRegistros(r.Registros)
	return r
}

// resposta é o que antes_de_responder deixa sair: o texto para o morador e se ele
// trocou a resposta original (bloqueio ou falha), em vez de só devolver os valores.
type resposta struct {
	texto   string
	trocada bool
}

// antesDeResponder devolve o texto do envio: com os valores do cofre no lugar dos
// marcadores, a mensagem da regra que bloqueou ou, se um fixo falhou, a FraseFalha.
func (s *Servico) antesDeResponder(ctx context.Context, sess *Sessao, t ganchos.Turno, texto string) resposta {
	alvo := sess.contato()
	r := t.AntesDeResponder(ctx, ganchos.AntesDeResponder{
		Agora: time.Now(), Texto: texto, AdministradoraID: alvo.AdministradoraID,
		CondominioID: alvo.CondominioID, Identidade: sess.identidadeDosGanchos(),
	})
	sess.anotarRegistros(r.Registros)
	switch {
	case bloqueioPorFalha(r.Registros):
		return resposta{texto: FraseFalha, trocada: true}
	case r.Decisao.Tipo == ganchos.Bloquear:
		return resposta{texto: r.Decisao.Motivo, trocada: true}
	}
	return resposta{texto: r.Evento.Texto}
}

// motivoDoTransbordo passa o transbordo ou o encerramento pelos ganchos, que nunca o
// impedem, e devolve o motivo sem PII.
func (s *Servico) motivoDoTransbordo(ctx context.Context, sess *Sessao, t ganchos.Turno, motivo string, encerramento bool) string {
	alvo := sess.contato()
	r := t.TransbordoOuEncerramento(ctx, ganchos.TransbordoOuEncerramento{
		Agora: time.Now(), Motivo: motivo, Encerramento: encerramento,
		AdministradoraID: alvo.AdministradoraID, CondominioID: alvo.CondominioID,
		Identidade: sess.identidadeDosGanchos(),
	})
	sess.anotarRegistros(r.Registros)
	return r.Evento.Motivo
}

// bloqueioPorFalha diz se um gancho falhou e a política fechou: o evento não pode seguir
// como estava, e o texto do evento pode ter ficado sem a máscara.
func bloqueioPorFalha(registros []ganchos.Registro) bool {
	return slices.ContainsFunc(registros, func(r ganchos.Registro) bool {
		return r.Falhou && r.Decisao.Tipo == ganchos.Bloquear
	})
}

// fatosSemPII descarta o fato com marcador: CPF e telefone nunca entram na memória.
func fatosSemPII(fatos []memoria.Fato) []memoria.Fato {
	return slices.DeleteFunc(fatos, func(f memoria.Fato) bool {
		return pii.TemMarcador(f.Chave) || pii.TemMarcador(f.Texto)
	})
}

func (sess *Sessao) identidadeDosGanchos() ganchos.Identidade {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	id := ganchos.Identidade{Cadastrado: sess.Identidade.Authenticated}
	if sess.Identidade.Role != "" {
		id.Papeis = []string{sess.Identidade.Role}
	}
	return id
}
