// Telas e rotas antigas, mantidas até a fase 2 do Studio cobrir todas as capacidades; a
// Task 31 do plano da fase 2 apaga este arquivo.
package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"go-concierge/internal/okf"
)

func (s *servidor) rotasLegado(mux *http.ServeMux) {
	mux.HandleFunc("GET /backdoor", s.telaAntiga("backdoor.html"))
	mux.HandleFunc("GET /backdoor/orquestracao", s.telaAntiga("orquestracao.html"))
	mux.HandleFunc("GET /api/anatomia", s.anatomia)
	mux.HandleFunc("POST /api/agentes", s.criar)
	mux.HandleFunc("DELETE /api/agente", s.excluir)
	mux.HandleFunc("POST /api/agentes/mover", s.mover)
}

func (s *servidor) anatomia(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"caixinhas":    okf.Caixinhas,
		"tipos_okf":    okf.TiposOKF,
		"raiz":         []string{okf.AgentYAML, okf.PersonaMD, okf.CasoYAML},
		"account":      s.account,
		"nexus_origem": s.nexus,
	})
}

func (s *servidor) criar(w http.ResponseWriter, r *http.Request) {
	var n okf.NovoAgente
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		writeErr(w, 400, err)
		return
	}
	rel, mud, err := s.cat.CriarAgente(n)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 201, map[string]any{"caminho": rel, "mudancas": mud})
}

func (s *servidor) excluir(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("agente")
	if err := s.cat.ExcluirSkill(rel); err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"excluido": rel})
}

// mover: a árvore do Backdoor ainda tem o drag-and-drop de mover/clonar agente, mas o
// catálogo não tem mais Mover (retirado nesta mesma fase, antes desta task, junto da
// limpeza que manteve o Admin no ar). A rota continua registrada para o Backdoor não
// quebrar ao carregar; o retorno é um erro claro em vez de fingir que moveu algo.
func (s *servidor) mover(w http.ResponseWriter, _ *http.Request) {
	writeErr(w, http.StatusGone, errors.New("mover agente: recurso retirado do catálogo nesta fase"))
}
