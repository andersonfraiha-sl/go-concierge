package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/okf"
)

// API do Studio: as skills do Assistente (textos, fluxo, criar, duplicar, excluir) e quem
// está usando (/api/eu). Nada é recusado por papel aqui: a Fundação embrulha estas rotas
// com a permissão de cada uma. O servidor só recusa fluxo inválido (422) e versão velha (409).

const (
	prazoCatalogo = 10 * time.Second
	maxCorpo      = 1 << 20
)

var errCorpoInvalido = errors.New("corpo inválido")

func (s *servidor) rotasStudio(mux *http.ServeMux) {
	mux.Handle("GET /api/eu", s.autenticado(http.HandlerFunc(s.studioEu)))
	mux.Handle("GET /api/skills", s.protegidaEmAlgum(acesso.SkillsLer, s.studioListar))
	mux.Handle("POST /api/skills", s.protegida(acesso.SkillsCriar, alvoSuperlogica, s.studioCriar))
	mux.Handle("GET /api/skills/{nome}", s.protegidaEmAlgum(acesso.SkillsLer, s.studioLerTextos))
	mux.Handle("PUT /api/skills/{nome}/textos", s.protegida(acesso.SkillsEditarTextos, alvoSuperlogica, s.studioSalvarTextos))
	mux.Handle("POST /api/skills/{nome}/duplicar", s.protegida(acesso.SkillsCriar, alvoSuperlogica, s.studioDuplicar))
	mux.Handle("DELETE /api/skills/{nome}", s.protegida(acesso.SkillsExcluir, alvoSuperlogica, s.studioExcluir))
	mux.Handle("GET /api/skills/{nome}/fluxo", s.protegidaEmAlgum(acesso.SkillsLer, s.studioLerFluxo))
	mux.Handle("PUT /api/skills/{nome}/fluxo", s.protegida(acesso.SkillsEditarFluxo, alvoSuperlogica, s.studioSalvarFluxo))
	mux.Handle("GET /studio/", studioEstatico(s.studio))
	mux.Handle("GET /studio", http.RedirectHandler("/studio/", http.StatusMovedPermanently))
}

// ---- /api/eu ----

type eu struct {
	ID               string   `json:"id"`
	Nome             string   `json:"nome"`
	Papel            string   `json:"papel"`
	Permissoes       []string `json:"permissoes"`
	AdministradoraID *string  `json:"administradoraId"` // null = Superlógica
	Dev              bool     `json:"dev"`
}

var errAdministradoraAmbigua = errors.New("usuário vê mais de uma administradora")

// montarEu descreve o usuário carregado. Só consulta as administradoras quando ele não é da
// Superlógica.
func montarEu(a acesso.Acesso, adms []admin.Administradora, dev bool) (eu, error) {
	quem := eu{
		ID: a.UsuarioID, Nome: a.Nome, Papel: acesso.PapelPrincipal(a), Dev: dev,
		Permissoes: make([]string, 0),
	}
	for _, p := range acesso.PermissoesEmAlgum(a) {
		quem.Permissoes = append(quem.Permissoes, string(p))
	}
	if acesso.Permitido(a, acesso.SkillsLer, acesso.Escopo{Tipo: acesso.TipoSuperlogica}) {
		return quem, nil
	}
	visiveis := administradorasVisiveis(a, adms)
	if len(visiveis) > 1 {
		return eu{}, errAdministradoraAmbigua
	}
	if len(visiveis) == 1 {
		quem.AdministradoraID = &visiveis[0].ID
	}
	return quem, nil
}

func (s *servidor) studioEu(w http.ResponseWriter, r *http.Request) {
	a, ok := acessoDe(r.Context())
	if !ok {
		responderNaoAutenticado(w)
		return
	}
	var adms []admin.Administradora
	if !acesso.Permitido(a, acesso.SkillsLer, acesso.Escopo{Tipo: acesso.TipoSuperlogica}) {
		m, err := s.admin.Ler(r.Context())
		if err != nil {
			s.responderErroInterno(w, r, "ler administradoras do usuário", err)
			return
		}
		adms = m.Administradoras
	}
	quem, err := montarEu(a, adms, s.authMode == authModeDev)
	if errors.Is(err, errAdministradoraAmbigua) {
		writeJSON(w, http.StatusConflict, erroAPI{Codigo: "ADMINISTRADORA_AMBIGUA", Mensagem: "Seu usuário está em mais de uma administradora. Peça para a Superlógica ajustar seu acesso."})
		return
	}
	writeJSON(w, http.StatusOK, quem)
}

// ---- skills ----

type skillResumo struct {
	casoDeUso
	Caminho string `json:"caminho"`
}

type nomeETitulo struct {
	Nome   string `json:"nome"`
	Titulo string `json:"titulo"`
}

// skillsDoAssistente é a resposta de GET /api/skills (SkillsSchema no Studio).
type skillsDoAssistente struct {
	Assistente string        `json:"assistente"`
	Skills     []skillResumo `json:"skills"`
}

// textosLidos é a resposta de GET /api/skills/{nome} (TextosLidosSchema no Studio).
type textosLidos struct {
	Caminho string     `json:"caminho"`
	Textos  okf.Textos `json:"textos"`
	Versao  string     `json:"versao"`
}

type textosParaGravar struct {
	Versao string     `json:"versao"`
	Textos okf.Textos `json:"textos"`
}

type fluxoParaGravar struct {
	Versao string    `json:"versao"`
	Fluxo  okf.Fluxo `json:"fluxo"`
}

type nomeCriado struct {
	Nome string `json:"nome"`
}

type versaoGravada struct {
	Versao string `json:"versao"`
}

func (s *servidor) studioListar(w http.ResponseWriter, r *http.Request) {
	casos, err := comPrazo(r, s.prazoDoCatalogo, s.casosDeUso)
	if err != nil {
		s.responderErro(w, fmt.Errorf("listar skills: %w", err))
		return
	}
	skills := make([]skillResumo, len(casos))
	for i, c := range casos {
		skills[i] = skillResumo{casoDeUso: c, Caminho: s.caminhoDaSkill(c.Nome)}
	}
	writeJSON(w, http.StatusOK, skillsDoAssistente{Assistente: s.atende.Agente, Skills: skills})
}

func (s *servidor) studioCriar(w http.ResponseWriter, r *http.Request) {
	corpo, err := lerCorpo[nomeETitulo](w, r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	_, err = comPrazo(r, s.prazoDoCatalogo, func() (string, error) { return s.cat.CriarSkill(s.atende.Agente, corpo.Nome, corpo.Titulo) })
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, nomeCriado{Nome: corpo.Nome})
}

func (s *servidor) studioLerTextos(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	lidos, err := comPrazo(r, s.prazoDoCatalogo, func() (textosLidos, error) {
		t, v, err := s.cat.LerTextos(rel)
		return textosLidos{Caminho: rel, Textos: t, Versao: v}, err
	})
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lidos)
}

func (s *servidor) studioSalvarTextos(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	corpo, err := lerCorpo[textosParaGravar](w, r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	versao, err := comPrazo(r, s.prazoDoCatalogo, func() (string, error) { return s.cat.SalvarTextos(rel, corpo.Versao, corpo.Textos) })
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusOK, versaoGravada{Versao: versao})
}

func (s *servidor) studioDuplicar(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	corpo, err := lerCorpo[nomeETitulo](w, r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	_, err = comPrazo(r, s.prazoDoCatalogo, func() (string, error) { return s.cat.DuplicarSkill(rel, corpo.Nome, corpo.Titulo) })
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, nomeCriado{Nome: corpo.Nome})
}

func (s *servidor) studioExcluir(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	if _, err := comPrazo(r, s.prazoDoCatalogo, func() (struct{}, error) { return struct{}{}, s.cat.ExcluirSkill(rel) }); err != nil {
		s.responderErro(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *servidor) studioLerFluxo(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	lido, err := comPrazo(r, s.prazoDoCatalogo, func() (okf.FluxoLido, error) { return s.cat.LerFluxo(rel) })
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lido)
}

func (s *servidor) studioSalvarFluxo(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	corpo, err := lerCorpo[fluxoParaGravar](w, r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	gravado, err := comPrazo(r, s.prazoDoCatalogo, func() (okf.FluxoGravado, error) { return s.cat.SalvarFluxo(rel, corpo.Versao, corpo.Fluxo) })
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusOK, gravado)
}

// ---- bordas ----

func (s *servidor) caminhoDaSkill(nome string) string {
	return s.atende.Agente + "/" + okf.DirAgentes + "/" + nome
}

// skillDoPedido devolve o caminho da skill {nome}; nome fora do padrão é skill que não existe.
func (s *servidor) skillDoPedido(r *http.Request) (string, error) {
	nome := r.PathValue("nome")
	if !okf.SlugOK(nome) {
		return "", okf.ErrSkillNaoExiste
	}
	return s.caminhoDaSkill(nome), nil
}

func lerCorpo[T any](w http.ResponseWriter, r *http.Request) (T, error) {
	var corpo T
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCorpo)).Decode(&corpo); err != nil {
		return corpo, fmt.Errorf("%w: %w", errCorpoInvalido, err)
	}
	return corpo, nil
}

// comPrazo roda o trabalho do catálogo (disco, às vezes o Nexus) com o prazo do pedido.
// prazo zero = sem teto além do contexto do pedido (os testes de handler: com o teto de
// parede, ler o catálogo recém-copiado numa máquina carregada estourava os 10 s).
// ponytail: o catálogo não recebe context; no estouro o trabalho segue até o fim em segundo
// plano e uma gravação pode valer mesmo com a resposta de erro (a versão protege a próxima).
// Passar o ctx para o Catalogo quando ele aceitar.
func comPrazo[T any](r *http.Request, prazo time.Duration, fazer func() (T, error)) (T, error) {
	ctx, cancel := r.Context(), context.CancelFunc(func() {})
	if prazo > 0 {
		ctx, cancel = context.WithTimeout(r.Context(), prazo)
	}
	defer cancel()
	type resultado struct {
		valor T
		err   error
	}
	feito := make(chan resultado, 1)
	go func() {
		// fora da goroutine do handler o net/http não segura o pânico: vira erro (500 com log)
		defer func() {
			if p := recover(); p != nil {
				feito <- resultado{err: fmt.Errorf("pânico no catálogo: %v", p)}
			}
		}()
		v, err := fazer()
		feito <- resultado{v, err}
	}()
	select {
	case res := <-feito:
		return res.valor, res.err
	case <-ctx.Done():
		var zero T
		return zero, fmt.Errorf("prazo do catálogo: %w", ctx.Err())
	}
}

// erroAPI é o formato de erro das rotas novas, o mesmo da Fundação.
type erroAPI struct {
	Codigo    string              `json:"codigo"`
	Mensagem  string              `json:"mensagem"`
	Problemas []okf.ProblemaFluxo `json:"problemas,omitempty"`
}

type erroConhecido struct {
	alvo   error
	status int
	erroAPI
}

// errosConhecidos fica numa função: tabela fixa, sem estado global.
func errosConhecidos() []erroConhecido {
	return []erroConhecido{
		{errCorpoInvalido, http.StatusBadRequest, erroAPI{Codigo: "CORPO_INVALIDO", Mensagem: "Não entendi o pedido."}},
		{okf.ErrNomeInvalido, http.StatusBadRequest, erroAPI{Codigo: "NOME_INVALIDO", Mensagem: "Use letras minúsculas, números e hífen."}},
		{okf.ErrSkillNaoExiste, http.StatusNotFound, erroAPI{Codigo: "SKILL_NAO_ENCONTRADA", Mensagem: "Esta skill não existe."}},
		{okf.ErrSemFluxo, http.StatusNotFound, erroAPI{Codigo: "SEM_FLUXO", Mensagem: "Esta skill não tem fluxo: ela responde pela base de conhecimento."}},
		{okf.ErrSkillJaExiste, http.StatusConflict, erroAPI{Codigo: "SKILL_JA_EXISTE", Mensagem: "Já existe uma skill com esse nome."}},
		{okf.ErrVersaoDesatualizada, http.StatusConflict, erroAPI{Codigo: "VERSAO_DESATUALIZADA", Mensagem: "Esta skill mudou desde que você abriu. Recarregue para continuar."}},
		{okf.ErrTituloVazio, http.StatusUnprocessableEntity, erroAPI{Codigo: "TITULO_VAZIO", Mensagem: "Dê um título para a skill."}},
		{okf.ErrVisibilidadeInvalida, http.StatusUnprocessableEntity, erroAPI{Codigo: "VISIBILIDADE_INVALIDA", Mensagem: "Escolha quem pode usar a skill."}},
	}
}

// responderErro traduz o erro do catálogo para a API; o que não é conhecido vai inteiro
// para o log e sai como mensagem fixa, sem detalhe interno.
func (s *servidor) responderErro(w http.ResponseWriter, err error) {
	var invalido *okf.ErrFluxoInvalido
	if errors.As(err, &invalido) {
		writeJSON(w, http.StatusUnprocessableEntity, erroAPI{Codigo: "FLUXO_INVALIDO", Mensagem: "O fluxo tem problemas; nada foi salvo.", Problemas: invalido.Problemas})
		return
	}
	for _, e := range errosConhecidos() {
		if errors.Is(err, e.alvo) {
			writeJSON(w, e.status, e.erroAPI)
			return
		}
	}
	if !errors.Is(err, context.Canceled) { // quem cancelou foi o cliente, que nem lê a resposta
		anotarFalha(context.Background(), w, fmt.Errorf("studio: %w", err), 1)
	}
	writeJSON(w, http.StatusInternalServerError, erroAPI{Codigo: "ERRO_INTERNO", Mensagem: "Não deu para concluir. Tente de novo."})
}
