package main

import "net/http"

// Dado de morador pela integração: memória e pedidos de um contato, sempre recortados por
// administradora (?administradora=, obrigatória, ou a fixa da implantação). Os handlers são os
// das rotas /api, que já não dependem do usuário do contexto: o escopo vem de
// administradoraPedida e o telefone sai mascarado do log da requisição (caminhoMascarado).
// Aqui só muda quem autoriza: a chave de serviço no lugar do RBAC.
func (s *servidor) rotasIntegracaoMorador(mux *http.ServeMux) {
	mux.HandleFunc("GET /integracao/v1/memorias", s.comChaveDeIntegracao(s.memoriasListar))
	mux.HandleFunc("GET /integracao/v1/memoria", s.comChaveDeIntegracao(s.memoriaLer))
	mux.HandleFunc("DELETE /integracao/v1/memoria", s.comChaveDeIntegracao(s.memoriaEsquecer))
	mux.HandleFunc("GET /integracao/v1/pedidos", s.comChaveDeIntegracao(s.pedidosLer))
}
