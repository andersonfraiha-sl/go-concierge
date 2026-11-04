package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"go-concierge/internal/admin"
	"go-concierge/internal/copiloto"
	"go-concierge/internal/harness"
	"go-concierge/internal/okf"
)

// API do Copiloto do Studio. POST /api/copiloto conversa com o modelo e devolve só as
// propostas que a pessoa pode aplicar na tela, com o motivo de cada recusa; o contexto
// mandado ao modelo é montado aqui, nunca vindo do cliente. GET /api/copiloto/estado diz
// se há modelo. O log nunca leva a conversa nem o contexto, só a forma do pedido.

const (
	prazoCopiloto        = 90 * time.Second
	maxMensagemCopiloto  = 4000
	maxHistoricoCopiloto = 20
	maxResumoProposta    = 90
	nivelErroDoFluxo     = "erro"
	permissaoAvancado    = "studio.avancado"
)

var (
	errPedidoCopiloto    = errors.New("pedido ao copiloto inválido")
	errSemAdministradora = errors.New("administradora vazia no usuário")
)

type pedidoAoCopiloto struct {
	Mensagem  string                       `json:"mensagem"`
	Historico []harness.MensagemDoCopiloto `json:"historico"`
	Contexto  copiloto.Contexto            `json:"contexto"`
}

type propostaDoCopiloto struct {
	ID       string            `json:"id"` // índice da operação na resposta do modelo, em texto
	Resumo   string            `json:"resumo"`
	Operacao copiloto.Operacao `json:"operacao"`
}

type respostaDoCopiloto struct {
	Mensagem  string               `json:"mensagem"`
	Propostas []propostaDoCopiloto `json:"propostas"`
	Recusas   []copiloto.Recusa    `json:"recusas"` // Indice no mesmo espaço do ID das propostas
}

type estadoDoCopiloto struct {
	Disponivel bool `json:"disponivel"`
}

func (s *servidor) rotasCopiloto(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/copiloto/estado", s.copilotoEstado)
	mux.HandleFunc("POST /api/copiloto", s.copilotoPerguntar)
}

func (s *servidor) copilotoEstado(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, estadoDoCopiloto{Disponivel: s.copiloto != nil})
}

// conversaRegistrada é o que vai para o log de cada pedido: a forma, nunca o conteúdo.
type conversaRegistrada struct {
	tela      copiloto.Tela
	propostas int
	recusas   int
	err       error
}

func (s *servidor) copilotoPerguntar(w http.ResponseWriter, r *http.Request) {
	inicio := time.Now()
	gravado := &statusGravado{ResponseWriter: w, status: http.StatusOK}
	reg := s.conversarComCopiloto(gravado, r)
	s.registrarConversa(r.Header.Get("X-Studio-Papel"), gravado.status, reg, time.Since(inicio))
}

func (s *servidor) conversarComCopiloto(w http.ResponseWriter, r *http.Request) conversaRegistrada {
	if s.copiloto == nil {
		writeJSON(w, http.StatusServiceUnavailable, erroAPI{Codigo: "COPILOTO_INDISPONIVEL", Mensagem: "O Copiloto está fora do ar. O resto do Studio continua funcionando."})
		return conversaRegistrada{}
	}
	usuario, ok := euDev(r.Header.Get("X-Studio-Papel"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, erroAPI{Codigo: "PAPEL_DESCONHECIDO", Mensagem: "Esse papel não existe."})
		return conversaRegistrada{}
	}
	quem, err := quemDoCopiloto(usuario)
	if err != nil {
		writeJSON(w, http.StatusForbidden, erroAPI{Codigo: "SEM_ACESSO", Mensagem: "Não deu para saber de qual administradora você é."})
		return conversaRegistrada{err: err}
	}
	corpo, err := lerCorpo[pedidoAoCopiloto](w, r)
	if err == nil {
		err = validarPedido(corpo)
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, erroAPI{Codigo: "PEDIDO_INVALIDO", Mensagem: "Não entendi o pedido."})
		return conversaRegistrada{tela: corpo.Contexto.Tela, err: err}
	}
	reg := conversaRegistrada{tela: corpo.Contexto.Tela}
	ret, err := s.retratoParaCopiloto(r, corpo.Contexto, usuario, quem)
	if err != nil {
		reg.err = err
		s.responderErroDoRetrato(w, err)
		return reg
	}
	ctx, cancel := context.WithTimeout(r.Context(), prazoCopiloto)
	defer cancel()
	resp, err := s.copiloto(ctx, harness.PedidoAoCopiloto{
		Mensagem: strings.TrimSpace(corpo.Mensagem), Historico: corpo.Historico,
		Contexto: ret.contexto, Operacoes: operacoesPermitidas(corpo.Contexto.Tela, quem),
	})
	if err != nil {
		reg.err = err
		responderErroDoModelo(w, err)
		return reg
	}
	_, recusas := copiloto.Filtrar(resp.Operacoes, corpo.Contexto, quem, ret.doEscopo)
	propostas, recusas := separarPropostas(resp.Operacoes, recusas, ret)
	reg.propostas, reg.recusas = len(propostas), len(recusas)
	writeJSON(w, http.StatusOK, respostaDoCopiloto{Mensagem: resp.Mensagem, Propostas: propostas, Recusas: recusas})
	return reg
}

// quemDoCopiloto: só a ausência de administradora é Superlógica; um id vazio é recusado,
// porque Administradora "" dá poder sobre o catálogo.
func quemDoCopiloto(u eu) (copiloto.Quem, error) {
	if u.AdministradoraID == nil {
		return copiloto.Quem{Permissoes: u.Permissoes}, nil
	}
	if *u.AdministradoraID == "" {
		return copiloto.Quem{}, errSemAdministradora
	}
	return copiloto.Quem{Permissoes: u.Permissoes, Administradora: *u.AdministradoraID}, nil
}

func validarPedido(p pedidoAoCopiloto) error {
	tamanho := utf8.RuneCountInString(strings.TrimSpace(p.Mensagem))
	switch {
	case tamanho == 0 || tamanho > maxMensagemCopiloto:
		return fmt.Errorf("%w: mensagem com %d caracteres", errPedidoCopiloto, tamanho)
	case len(p.Historico) > maxHistoricoCopiloto:
		return fmt.Errorf("%w: histórico com %d mensagens", errPedidoCopiloto, len(p.Historico))
	case !telaConhecida(p.Contexto.Tela):
		return fmt.Errorf("%w: tela %q", errPedidoCopiloto, p.Contexto.Tela)
	case faltaSelecao(p.Contexto):
		return fmt.Errorf("%w: tela %s sem seleção", errPedidoCopiloto, p.Contexto.Tela)
	}
	for _, m := range p.Historico {
		if m.Papel != harness.PapelPessoa && m.Papel != harness.PapelCopiloto {
			return fmt.Errorf("%w: papel %q no histórico", errPedidoCopiloto, m.Papel)
		}
	}
	return nil
}

func telaConhecida(t copiloto.Tela) bool {
	return slices.Contains([]copiloto.Tela{
		copiloto.TelaSkills, copiloto.TelaSkill, copiloto.TelaFluxo, copiloto.TelaCondominios,
		copiloto.TelaCondominio, copiloto.TelaConcierge, copiloto.TelaFerramentas, copiloto.TelaAvancado,
	}, t)
}

func faltaSelecao(c copiloto.Contexto) bool {
	switch c.Tela {
	case copiloto.TelaSkill, copiloto.TelaFluxo:
		return c.Skill == ""
	case copiloto.TelaCondominio:
		return c.Condominio == ""
	}
	return false
}

// operacoesPermitidas é o que a tela aceita e a pessoa tem permissão de fazer; o resto o
// modelo nem conhece (Filtrar confere de novo depois).
func operacoesPermitidas(t copiloto.Tela, quem copiloto.Quem) []copiloto.TipoDeOperacao {
	return slices.DeleteFunc(copiloto.OperacoesDaTela(t), func(tipo copiloto.TipoDeOperacao) bool {
		return !slices.Contains(quem.Permissoes, copiloto.PermissaoDe(tipo))
	})
}

// ---- contexto montado no servidor ----

// retrato é o que o servidor sabe na hora do pedido: o contexto para o modelo e os nomes,
// o escopo e a classificação de ferramentas para conferir a resposta.
type retrato struct {
	contexto    string
	skills      map[string]string // nome → título
	condominios map[string]string // id → nome, só os do escopo
	escrita     func(recurso string) bool
}

func (r retrato) doEscopo(id string) bool {
	_, ok := r.condominios[id]
	return ok
}

func (s *servidor) retratoParaCopiloto(r *http.Request, tela copiloto.Contexto, usuario eu, quem copiloto.Quem) (retrato, error) {
	mapa, err := s.admin.Ler()
	if err != nil {
		return retrato{}, fmt.Errorf("copiloto: ler condomínios: %w", err)
	}
	visao := mapa.Visao(quem.Administradora) // "" só para a Superlógica (quemDoCopiloto)
	return comPrazo(r, func() (retrato, error) {
		casos, err := s.casosDeUso()
		if err != nil {
			return retrato{}, fmt.Errorf("copiloto: listar skills: %w", err)
		}
		contexto, err := s.contextoDaTela(tela, usuario, visao, casos)
		if err != nil {
			return retrato{}, err
		}
		return retrato{
			contexto: contexto, skills: titulosDasSkills(casos),
			condominios: nomesDosCondominios(visao.Condominios), escrita: escritaDoCatalogo(s.cat.RecursosNexus()),
		}, nil
	})
}

func (s *servidor) contextoDaTela(tela copiloto.Contexto, usuario eu, visao admin.Mapeamento, casos []casoDeUso) (string, error) {
	cabecalho := "Tela: " + string(tela.Tela) + "\n"
	var conteudo any
	switch tela.Tela {
	case copiloto.TelaSkill, copiloto.TelaFluxo:
		skill, err := s.skillParaCopiloto(tela.Skill)
		if err != nil {
			return "", err
		}
		conteudo = skill
	case copiloto.TelaCondominio:
		conteudo = condominioParaCopiloto(visao, tela.Condominio, casos)
	case copiloto.TelaCondominios:
		conteudo = map[string]any{"condominios": nomesDosCondominios(visao.Condominios), "skills": resumosDasSkills(casos)}
	case copiloto.TelaSkills:
		conteudo = map[string]any{"skills": resumosDasSkills(casos)}
	case copiloto.TelaAvancado:
		arquivo, err := s.arquivoParaCopiloto(tela, usuario)
		if err != nil {
			return "", err
		}
		return cabecalho + arquivo, nil
	default:
		return cabecalho, nil
	}
	raw, err := json.Marshal(conteudo)
	if err != nil {
		return "", fmt.Errorf("copiloto: montar contexto da tela %s: %w", tela.Tela, err)
	}
	return cabecalho + string(raw), nil
}

type skillNoContexto struct {
	Nome      string              `json:"nome"`
	Textos    okf.Textos          `json:"textos"`
	Fluxo     *okf.Fluxo          `json:"fluxo,omitempty"` // nil: a skill responde pela base de conhecimento
	Problemas []okf.ProblemaFluxo `json:"problemas,omitempty"`
}

func (s *servidor) skillParaCopiloto(nome string) (skillNoContexto, error) {
	if !okf.SlugOK(nome) {
		return skillNoContexto{}, fmt.Errorf("copiloto: montar contexto da skill %s: %w", nome, okf.ErrSkillNaoExiste)
	}
	rel := s.caminhoDaSkill(nome)
	textos, _, err := s.cat.LerTextos(rel)
	if err != nil {
		return skillNoContexto{}, fmt.Errorf("copiloto: montar contexto da skill %s: %w", nome, err)
	}
	skill := skillNoContexto{Nome: nome, Textos: textos}
	lido, err := s.cat.LerFluxo(rel)
	switch {
	case errors.Is(err, okf.ErrSemFluxo):
		return skill, nil
	case err != nil:
		return skillNoContexto{}, fmt.Errorf("copiloto: montar contexto da skill %s: %w", nome, err)
	}
	skill.Fluxo, skill.Problemas = &lido.Fluxo, lido.Problemas
	return skill, nil
}

type skillResumida struct {
	Nome   string `json:"nome"`
	Titulo string `json:"titulo"`
	Resumo string `json:"resumo"`
}

func resumosDasSkills(casos []casoDeUso) []skillResumida {
	out := make([]skillResumida, len(casos))
	for i, c := range casos {
		out[i] = skillResumida{Nome: c.Nome, Titulo: c.Titulo, Resumo: c.Resumo}
	}
	return out
}

// condominioParaCopiloto lê só da visão do escopo: condomínio de outra administradora
// não aparece, nem que o cliente mande o id.
func condominioParaCopiloto(visao admin.Mapeamento, id string, casos []casoDeUso) any {
	c, ok := visao.Condominio(id)
	if !ok {
		return map[string]string{"aviso": "Este condomínio não está com quem pede."}
	}
	return map[string]any{
		"condominio":          map[string]string{"id": c.ID, "nome": c.Nome},
		"skills":              resumosDasSkills(casos),
		"habilitacoes":        visao.Habilitacoes[c.ID],
		"habilitacoes_padrao": visao.Habilitacoes[admin.Padrao],
		"orientacao":          visao.Orientacoes[c.ID],
		"ajustes":             visao.Payloads[c.ID],
	}
}

func (s *servidor) arquivoParaCopiloto(tela copiloto.Contexto, usuario eu) (string, error) {
	if !slices.Contains(usuario.Permissoes, permissaoAvancado) {
		return "Sem acesso ao Avançado.", nil
	}
	if tela.Agente == "" || tela.Caminho == "" {
		return "Nenhum arquivo aberto.", nil
	}
	conteudo, err := s.cat.LerArquivo(tela.Agente, tela.Caminho)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Sprintf("Arquivo %s/%s, novo e vazio.", tela.Agente, tela.Caminho), nil
	case err != nil:
		return "", fmt.Errorf("%w: ler arquivo %s/%s: %w", errPedidoCopiloto, tela.Agente, tela.Caminho, err)
	}
	return fmt.Sprintf("Arquivo %s/%s:\n%s", tela.Agente, tela.Caminho, conteudo), nil
}

func titulosDasSkills(casos []casoDeUso) map[string]string {
	out := make(map[string]string, len(casos))
	for _, c := range casos {
		out[c.Nome] = c.Titulo
	}
	return out
}

func nomesDosCondominios(condominios []admin.Condominio) map[string]string {
	out := make(map[string]string, len(condominios))
	for _, c := range condominios {
		out[c.ID] = c.Nome
	}
	return out
}

// escritaDoCatalogo tira um retrato de quais ferramentas gravam, como o catálogo faz ao
// gravar o fluxo: fora do catálogo, deduz pelo nome.
func escritaDoCatalogo(recursos []okf.Recurso) func(string) bool {
	gravam := make(map[string]bool, len(recursos))
	for _, r := range recursos {
		gravam[r.Name] = r.Write
	}
	return func(nome string) bool {
		if escrita, ok := gravam[nome]; ok {
			return escrita
		}
		return okf.EscritaPeloNome(nome)
	}
}

// ---- resposta ----

// separarPropostas junta às recusas de Filtrar os fluxos com erro e numera as propostas
// pelo índice da operação, o mesmo das recusas.
func separarPropostas(ops []copiloto.Operacao, recusasDoFiltro []copiloto.Recusa, ret retrato) ([]propostaDoCopiloto, []copiloto.Recusa) {
	recusas := slices.Clone(recusasDoFiltro)
	if recusas == nil {
		recusas = []copiloto.Recusa{}
	}
	recusada := make(map[int]bool, len(recusas))
	for _, r := range recusas {
		recusada[r.Indice] = true
	}
	propostas := []propostaDoCopiloto{}
	for i, op := range ops {
		if recusada[i] {
			continue
		}
		if motivo := motivoDoFluxo(op, ret.escrita); motivo != "" {
			recusas = append(recusas, copiloto.Recusa{Indice: i, Motivo: motivo})
			continue
		}
		propostas = append(propostas, propostaDoCopiloto{ID: strconv.Itoa(i), Resumo: resumoDaProposta(op, ret), Operacao: op})
	}
	slices.SortFunc(recusas, func(a, b copiloto.Recusa) int { return a.Indice - b.Indice })
	return propostas, recusas
}

// motivoDoFluxo: todo trocar_fluxo aceito passa pela mesma validação da gravação; é o que
// impede propor um Gravar sem Confirmar antes.
func motivoDoFluxo(op copiloto.Operacao, escrita func(string) bool) string {
	if op.Tipo != copiloto.TrocarFluxo || op.Fluxo == nil {
		return ""
	}
	problemas := copiloto.ValidarFluxoProposto(*op.Fluxo, escrita)
	i := slices.IndexFunc(problemas, func(p okf.ProblemaFluxo) bool { return p.Nivel == nivelErroDoFluxo })
	if i < 0 {
		return ""
	}
	return "A proposta deixa o fluxo com erro: " + problemas[i].Mensagem
}

func resumoDaProposta(op copiloto.Operacao, ret retrato) string {
	skill := nomeOuChave(ret.skills, op.Skill)
	condominio := nomeOuChave(ret.condominios, op.Condominio)
	var resumo string
	switch op.Tipo {
	case copiloto.PreencherTextos:
		resumo = "Mudar os textos de " + skill
	case copiloto.TrocarFluxo:
		resumo = "Mudar o fluxo de " + skill
	case copiloto.CriarSkill:
		resumo = "Criar a skill " + op.Titulo
	case copiloto.ExcluirSkill:
		resumo = "Excluir a skill " + skill
	case copiloto.HabilitarSkill:
		resumo = verboDeHabilitar(op.Habilitado) + " " + skill + " em " + condominio
	case copiloto.Orientar:
		resumo = "Orientar o Concierge em " + condominio
		if op.Skill != "" {
			resumo = "Orientar " + skill + " em " + condominio
		}
	case copiloto.Ajustar:
		resumo = "Mudar os ajustes de " + condominio
	case copiloto.EditarPersona:
		resumo = "Mudar o jeito de falar do Concierge"
	case copiloto.EditarArquivo:
		resumo = "Editar " + op.Caminho
	default:
		resumo = "Proposta do Copiloto"
	}
	return cortarResumo(resumo)
}

func verboDeHabilitar(habilitado *bool) string {
	if habilitado != nil && *habilitado {
		return "Ligar"
	}
	return "Desligar"
}

func nomeOuChave(nomes map[string]string, chave string) string {
	if nome := nomes[chave]; nome != "" {
		return nome
	}
	return chave
}

func cortarResumo(s string) string {
	r := []rune(s)
	if len(r) <= maxResumoProposta {
		return s
	}
	return string(r[:maxResumoProposta-1]) + "…"
}

// ---- erros e log ----

func (s *servidor) responderErroDoRetrato(w http.ResponseWriter, err error) {
	if errors.Is(err, errPedidoCopiloto) {
		writeJSON(w, http.StatusBadRequest, erroAPI{Codigo: "PEDIDO_INVALIDO", Mensagem: "Não entendi o pedido."})
		return
	}
	s.responderErro(w, err)
}

func responderErroDoModelo(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		writeJSON(w, http.StatusGatewayTimeout, erroAPI{Codigo: "COPILOTO_DEMOROU", Mensagem: "O Copiloto demorou demais para responder. Tente de novo."})
	case errors.Is(err, context.Canceled):
		// quem cancelou foi o cliente, que nem lê a resposta
	default:
		writeJSON(w, http.StatusInternalServerError, erroAPI{Codigo: "ERRO_INTERNO", Mensagem: "Não deu para concluir. Tente de novo."})
	}
}

func (s *servidor) registrarConversa(papel string, status int, reg conversaRegistrada, duracao time.Duration) {
	atributos := []any{
		"operacao", "copiloto.perguntar", "durationMs", duracao.Milliseconds(), "status", status,
		"papel", papel, "tela", string(reg.tela), "propostas", reg.propostas, "recusas", reg.recusas,
	}
	nivel := slog.LevelInfo
	if reg.err != nil {
		atributos = append(atributos, "erro", reg.err.Error())
	}
	if status >= http.StatusInternalServerError {
		nivel = slog.LevelError
	}
	slog.New(slog.NewJSONHandler(s.log.Writer(), nil)).Log(context.Background(), nivel, "copiloto", atributos...)
}

// statusGravado guarda o status escrito, para o log.
type statusGravado struct {
	http.ResponseWriter
	status int
}

func (g *statusGravado) WriteHeader(status int) {
	g.status = status
	g.ResponseWriter.WriteHeader(status)
}
