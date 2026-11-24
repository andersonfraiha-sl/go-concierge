package conciergeapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"go-concierge/internal/acesso"
	"go-concierge/internal/core/api/execucao"
	"go-concierge/internal/core/catalogo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	"go-concierge/internal/identidade"
)

func (s *servidor) configuracaoDeExecucao(w http.ResponseWriter, r *http.Request) {
	a, ok := acessoDe(r.Context())
	if !ok || !acesso.Permitido(a, acesso.StudioAvancado, acesso.Escopo{Tipo: acesso.TipoSuperlogica}) {
		responderSemPermissao(w)
		return
	}
	company := strings.TrimSpace(r.URL.Query().Get("company_id"))
	id, err := uuid.Parse(r.URL.Query().Get("channel_id"))
	if err != nil || company == "" {
		writeErr(w, http.StatusBadRequest, canais.ErrInvalido)
		return
	}
	if !s.implantacao.Permite(company) {
		responderSemPermissao(w)
		return
	}
	if s.atende == nil || s.atende.Canais == nil {
		writeErr(w, http.StatusServiceUnavailable, canais.ErrDesconhecido)
		return
	}
	canal, err := s.atende.Canais.PorID(r.Context(), company, id)
	if err != nil {
		if errors.Is(err, canais.ErrDesconhecido) {
			writeErr(w, http.StatusNotFound, canais.ErrDesconhecido)
		} else {
			s.responderErroInterno(w, r, "ler canal de execução", err)
		}
		return
	}
	if canal.CompanyID != company || canal.ID != id {
		responderSemPermissao(w)
		return
	}
	if !canal.Habilitado {
		writeErr(w, http.StatusConflict, canais.ErrDesabilitado)
		return
	}
	casos, err := s.atende.Canais.Casos(r.Context(), canal)
	if err != nil {
		s.responderErroInterno(w, r, "ler casos de execução", err)
		return
	}
	ctxDados := atende.ComEscopoSessao(identidade.ComoRuntime(r.Context()), company, id)
	if s.conteudoExecucao == nil || s.catalogoExecucao == nil {
		writeErr(w, http.StatusServiceUnavailable, errors.New("API de dados indisponível"))
		return
	}
	conteudo, err := s.conteudoExecucao.Conteudo(ctxDados, company, canal.CondoID)
	if err != nil {
		s.responderErroInterno(w, r, "ler conteúdo de execução", err)
		return
	}
	arquivos, err := s.catalogoExecucao.Listar(r.Context())
	if err != nil {
		s.responderErroInterno(w, r, "ler catálogo de execução", err)
		return
	}
	if casos == nil {
		casos = []canais.Caso{}
	}
	if arquivos == nil {
		arquivos = []catalogo.Arquivo{}
	}
	publico := canais.Canal{ID: canal.ID, CompanyID: canal.CompanyID, CondoID: canal.CondoID, Nome: canal.Nome, Modo: canal.Modo, Habilitado: canal.Habilitado, Agente: canal.Agente, Integracoes: canal.Integracoes}
	writeJSON(w, http.StatusOK, execucao.Configuracao{Canal: publico, Casos: casos, Conteudo: conteudo, Arquivos: arquivos})
}
