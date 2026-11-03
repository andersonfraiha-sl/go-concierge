package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"go-concierge/internal/okf"
)

// API do Studio: as skills do Assistente (textos, fluxo, criar, duplicar, excluir) e quem
// está usando (/api/eu). Nada é recusado por papel aqui: a Fundação embrulha estas rotas
// com a permissão de cada uma. O servidor só recusa fluxo inválido (422) e versão velha (409).

const (
	prazoCatalogo = 10 * time.Second
	maxCorpo      = 1 << 20
)

var (
	errCorpoInvalido      = errors.New("corpo inválido")
	errSkillNaoEncontrada = errors.New("skill não encontrada")
)

func (s *servidor) rotasStudio(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/eu", s.studioEu)
	mux.HandleFunc("GET /api/skills", s.studioListar)
	mux.HandleFunc("POST /api/skills", s.studioCriar)
	mux.HandleFunc("GET /api/skills/{nome}", s.studioLerTextos)
	mux.HandleFunc("PUT /api/skills/{nome}/textos", s.studioSalvarTextos)
	mux.HandleFunc("POST /api/skills/{nome}/duplicar", s.studioDuplicar)
	mux.HandleFunc("DELETE /api/skills/{nome}", s.studioExcluir)
	mux.HandleFunc("GET /api/skills/{nome}/fluxo", s.studioLerFluxo)
	mux.HandleFunc("PUT /api/skills/{nome}/fluxo", s.studioSalvarFluxo)
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

// euDev é o usuário de desenvolvimento para o papel do cabeçalho X-Studio-Papel, até a
// Fundação entregar o acesso de verdade.
func euDev(papel string) (eu, bool) {
	adm := "adm-exemplo"
	gestor := []string{
		"skills.ler", "conversas.ver", "conversas.atender", "pedidos.ver", "memoria.ver",
		"condominios.habilitar", "skills.editar_textos", "pedidos.editar", "memoria.esquecer",
	}
	switch papel {
	case "", "construtor":
		return eu{ID: "dev-superlogica", Nome: "Dev Superlógica", Papel: "admin_superlogica", Dev: true, Permissoes: []string{
			"skills.ler", "skills.editar_textos", "skills.editar_fluxo", "skills.criar", "skills.excluir",
			"ferramentas.editar", "condominios.habilitar", "condominios.editar", "conversas.ver",
			"conversas.atender", "pedidos.ver", "pedidos.editar", "memoria.ver", "memoria.esquecer",
			"administradoras.editar", "usuarios.editar", "studio.avancado",
		}}, true
	case "gestor":
		return eu{ID: "dev-gestor", Nome: "Dev Gestor", Papel: "gestor_administradora", Permissoes: gestor, AdministradoraID: &adm, Dev: true}, true
	case "gestor_avancado":
		return eu{ID: "dev-gestor", Nome: "Dev Gestor", Papel: "gestor_administradora", Permissoes: append(gestor, "studio.avancado"), AdministradoraID: &adm, Dev: true}, true
	}
	return eu{}, false
}

func (s *servidor) studioEu(w http.ResponseWriter, r *http.Request) {
	quem, ok := euDev(r.Header.Get("X-Studio-Papel"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, erroAPI{Codigo: "PAPEL_DESCONHECIDO", Mensagem: "Esse papel não existe."})
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

func (s *servidor) studioListar(w http.ResponseWriter, r *http.Request) {
	casos, err := comPrazo(r, s.casosDeUso)
	if err != nil {
		s.responderErro(w, fmt.Errorf("listar skills: %w", err))
		return
	}
	skills := make([]skillResumo, len(casos))
	for i, c := range casos {
		skills[i] = skillResumo{casoDeUso: c, Caminho: s.caminhoDaSkill(c.Nome)}
	}
	writeJSON(w, http.StatusOK, map[string]any{"assistente": s.atende.Agente, "skills": skills})
}

func (s *servidor) studioCriar(w http.ResponseWriter, r *http.Request) {
	var corpo nomeETitulo
	if err := lerCorpo(w, r, &corpo); err != nil {
		s.responderErro(w, err)
		return
	}
	_, err := comPrazo(r, func() (string, error) { return s.cat.CriarSkill(s.atende.Agente, corpo.Nome, corpo.Titulo) })
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"nome": corpo.Nome})
}

func (s *servidor) studioLerTextos(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	type lidos struct {
		textos okf.Textos
		versao string
	}
	l, err := comPrazo(r, func() (lidos, error) {
		t, v, err := s.cat.LerTextos(rel)
		return lidos{t, v}, err
	})
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"caminho": rel, "textos": l.textos, "versao": l.versao})
}

func (s *servidor) studioSalvarTextos(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	var corpo struct {
		Versao string     `json:"versao"`
		Textos okf.Textos `json:"textos"`
	}
	if err := lerCorpo(w, r, &corpo); err != nil {
		s.responderErro(w, err)
		return
	}
	versao, err := comPrazo(r, func() (string, error) { return s.cat.SalvarTextos(rel, corpo.Versao, corpo.Textos) })
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"versao": versao})
}

func (s *servidor) studioDuplicar(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	var corpo nomeETitulo
	if err := lerCorpo(w, r, &corpo); err != nil {
		s.responderErro(w, err)
		return
	}
	_, err = comPrazo(r, func() (string, error) { return s.cat.DuplicarSkill(rel, corpo.Nome, corpo.Titulo) })
	if err != nil {
		s.responderErro(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"nome": corpo.Nome})
}

func (s *servidor) studioExcluir(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	if _, err := comPrazo(r, func() ([]string, error) { return s.excluirSkill(rel) }); err != nil {
		s.responderErro(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// excluirSkill confere que a pasta é uma skill antes de apagar: ExcluirAgente não distingue "não existe".
func (s *servidor) excluirSkill(rel string) ([]string, error) {
	dir, err := s.cat.ResolveAgente(rel)
	if err != nil {
		return nil, fmt.Errorf("excluir skill %s: %w", rel, err)
	}
	if !okf.IsAgent(dir) {
		return nil, errSkillNaoEncontrada
	}
	mudancas, err := s.cat.ExcluirAgente(rel)
	if err != nil {
		return nil, fmt.Errorf("excluir skill %s: %w", rel, err)
	}
	return mudancas, nil
}

func (s *servidor) studioLerFluxo(w http.ResponseWriter, r *http.Request) {
	rel, err := s.skillDoPedido(r)
	if err != nil {
		s.responderErro(w, err)
		return
	}
	lido, err := comPrazo(r, func() (okf.FluxoLido, error) { return s.cat.LerFluxo(rel) })
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
	var corpo struct {
		Versao string    `json:"versao"`
		Fluxo  okf.Fluxo `json:"fluxo"`
	}
	if err := lerCorpo(w, r, &corpo); err != nil {
		s.responderErro(w, err)
		return
	}
	gravado, err := comPrazo(r, func() (okf.FluxoGravado, error) { return s.cat.SalvarFluxo(rel, corpo.Versao, corpo.Fluxo) })
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
		return "", errSkillNaoEncontrada
	}
	return s.caminhoDaSkill(nome), nil
}

func lerCorpo(w http.ResponseWriter, r *http.Request, destino any) error {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCorpo)).Decode(destino); err != nil {
		return fmt.Errorf("%w: %w", errCorpoInvalido, err)
	}
	return nil
}

// comPrazo roda o trabalho do catálogo (disco, às vezes o Nexus) com o prazo do pedido.
// ponytail: o catálogo não recebe context; no estouro o trabalho segue até o fim em segundo
// plano e uma gravação pode valer mesmo com a resposta de erro (a versão protege a próxima).
// Passar o ctx para o Catalogo quando ele aceitar.
func comPrazo[T any](r *http.Request, fazer func() (T, error)) (T, error) {
	ctx, cancel := context.WithTimeout(r.Context(), prazoCatalogo)
	defer cancel()
	type resultado struct {
		valor T
		err   error
	}
	feito := make(chan resultado, 1)
	go func() {
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
		{errSkillNaoEncontrada, http.StatusNotFound, erroAPI{Codigo: "SKILL_NAO_ENCONTRADA", Mensagem: "Esta skill não existe."}},
		{fs.ErrNotExist, http.StatusNotFound, erroAPI{Codigo: "SKILL_NAO_ENCONTRADA", Mensagem: "Esta skill não existe."}},
		{okf.ErrSemFluxo, http.StatusNotFound, erroAPI{Codigo: "SEM_FLUXO", Mensagem: "Esta skill não tem fluxo: ela responde pela base de conhecimento."}},
		{okf.ErrSkillJaExiste, http.StatusConflict, erroAPI{Codigo: "SKILL_JA_EXISTE", Mensagem: "Já existe uma skill com esse nome."}},
		{okf.ErrVersaoDesatualizada, http.StatusConflict, erroAPI{Codigo: "VERSAO_DESATUALIZADA", Mensagem: "Esta skill mudou desde que você abriu. Recarregue para continuar."}},
		{okf.ErrTituloVazio, http.StatusUnprocessableEntity, erroAPI{Codigo: "TITULO_VAZIO", Mensagem: "Dê um título para a skill."}},
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
	s.log.Printf("studio: %v", err)
	writeJSON(w, http.StatusInternalServerError, erroAPI{Codigo: "ERRO_INTERNO", Mensagem: "Não deu para concluir. Tente de novo."})
}
