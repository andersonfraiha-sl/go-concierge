package atende

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/metric/noop"

	"go-concierge/internal/admin"
	"go-concierge/internal/entrada"
	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/auditoria"
	"go-concierge/internal/ganchos/fixos"
	"go-concierge/internal/ganchos/pii"
	"go-concierge/internal/memoria"
)

// MotivoFalhaDeGancho é o motivo do transbordo quando um fixo de mensagem_recebida
// falha: a mensagem não pode seguir sem a checagem e a máscara (spec §11).
const MotivoFalhaDeGancho = "não consegui atender agora"

// ganchos é o conjunto do serviço, validado uma vez: Servico.Ganchos, senão
// Config.Ganchos, senão só os fixos. Conjunto injetado sem algum fixo é recusado.
func (s *Servico) ganchos() (*ganchos.Conjunto, error) {
	s.ganchosUmaVez.Do(func() { s.conjunto, s.errGanchos = conjuntoDoServico(s.Ganchos, s.Config.Ganchos) })
	return s.conjunto, s.errGanchos
}

func conjuntoDoServico(injetados ...*ganchos.Conjunto) (*ganchos.Conjunto, error) {
	for _, c := range injetados {
		if c == nil {
			continue
		}
		if faltam := fixos.Ausentes(c.NomesDosFixos()); len(faltam) > 0 {
			return nil, fmt.Errorf("atende: ganchos sem os fixos %s", strings.Join(faltam, ", "))
		}
		return c, nil
	}
	return conjuntoDosFixos(), nil
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

// abrirTurno abre o turno dos ganchos da sessão: o cofre numera a partir do marcador que
// a conversa guardou. Quem abre chama sess.fecharTurno no fim.
func (s *Servico) abrirTurno(sess *Sessao) (ganchos.Turno, error) {
	conjunto, err := s.ganchos()
	if err != nil {
		return ganchos.Turno{}, err
	}
	return conjunto.TurnoAPartirDe(sess.proximoMarcador()), nil
}

// mascararForaDoTurno mascara a fala que chega sem turno (atendimento humano, revisão
// trocada): ela vai ao histórico, e o histórico volta ao modelo depois. Só o cofre, sem
// mensagem_recebida, que registraria um bloqueio de escopo falso.
func (s *Servico) mascararForaDoTurno(sess *Sessao, texto string) (string, error) {
	t, err := s.abrirTurno(sess)
	if err != nil {
		return "", err
	}
	defer sess.fecharTurno(t)
	return mascararEntrada(t, texto), nil
}

// mascararEntrada é o que o pii_na_entrada faz: marcador digitado perde a forma, e CPF e
// telefone viram marcadores do cofre do turno.
func mascararEntrada(t ganchos.Turno, texto string) string {
	return mascarado(t.Cofre(), pii.Neutralizar(texto))
}

// mascarado é o texto com CPF e telefone trocados por marcadores do cofre: o que vai ao
// modelo, ao turno e ao histórico; o valor real fica só no cofre e no envio.
func mascarado(c *pii.Cofre, texto string) string {
	m, _ := c.Mascarar(texto)
	return m
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
		Agora: s.agora(), Texto: texto, CompanyID: msg.CompanyID, Tenant: msg.Tenant,
		Mapeamento: m, Implantacao: s.Implantacao, ContatoDaSessao: sess.contato(),
		Identidade: sess.identidadeDosGanchos(),
	})
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
		Agora: s.agora(), Texto: texto, AdministradoraID: alvo.AdministradoraID,
		CondominioID: alvo.CondominioID, Identidade: sess.identidadeDosGanchos(),
	})
	switch {
	case bloqueioPorFalha(r.Registros):
		return resposta{texto: FraseFalha, trocada: true}
	case r.Decisao.Tipo == ganchos.Bloquear:
		return resposta{texto: r.Decisao.Motivo, trocada: true}
	}
	return resposta{texto: r.Evento.Texto}
}

// respostaPronta prepara uma resposta escrita pelo código (não pelo modelo), que pode trazer
// valores reais: o envio sai do antes_de_responder; o turno e o histórico guardam o texto
// mascarado, ou o que o gancho pôs no lugar.
func (s *Servico) respostaPronta(ctx context.Context, sess *Sessao, t ganchos.Turno, texto string) (enviar, guardar string) {
	guardar = mascarado(t.Cofre(), texto)
	saida := s.antesDeResponder(ctx, sess, t, guardar)
	if saida.trocada {
		return saida.texto, saida.texto
	}
	return saida.texto, guardar
}

// transbordoOuEncerramento passa o transbordo ou o encerramento pelos ganchos, que nunca o
// impedem; o motivo do resultado sai sem PII.
func (s *Servico) transbordoOuEncerramento(ctx context.Context, sess *Sessao, t ganchos.Turno, motivo string, encerramento bool) ganchos.Resultado[ganchos.TransbordoOuEncerramento] {
	alvo := sess.contato()
	return t.TransbordoOuEncerramento(ctx, ganchos.TransbordoOuEncerramento{
		Agora: s.agora(), Motivo: motivo, Encerramento: encerramento,
		AdministradoraID: alvo.AdministradoraID, CondominioID: alvo.CondominioID,
		Identidade: sess.identidadeDosGanchos(),
	})
}

// motivoDoTransbordo saneia o motivo de um transbordo do turno.
func (s *Servico) motivoDoTransbordo(ctx context.Context, sess *Sessao, t ganchos.Turno, motivo string) string {
	r := s.transbordoOuEncerramento(ctx, sess, t, motivo, false)
	return r.Evento.Motivo
}

// encerramento passa o encerramento pelos ganchos. Roda fora do turno (o canal fecha o
// protocolo enquanto um turno pode estar em curso): sem o coletor do turno no ctx, a
// auditoria grava na hora, sem misturar com a do turno.
func (s *Servico) encerramento(ctx context.Context, sess *Sessao) {
	ctx = comAuditoria(ctx, sess, "")
	t, err := s.abrirTurno(sess)
	if err != nil {
		slog.ErrorContext(ctx, "atende: encerramento sem ganchos", "operacao", "encerrar", "sessao", sess.ID, "erro", err)
		return
	}
	s.transbordoOuEncerramento(ctx, sess, t, "", true)
}

// comAuditoria põe no ctx a sessão e o usuário que a auditoria dos ganchos grava; o alvo e a
// hora vêm de cada avaliação. usuario é o atendente (vazio quando quem fala é o morador).
func comAuditoria(ctx context.Context, sess *Sessao, usuario string) context.Context {
	return auditoria.ComBase(ctx, auditoria.Linha{
		Origem: ganchos.OrigemConcierge, SessaoID: sess.ID, Protocolo: sess.Protocolo, UsuarioID: usuario,
	})
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
