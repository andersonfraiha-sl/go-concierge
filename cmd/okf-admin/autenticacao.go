package main

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go-concierge/internal/acesso"
	"go-concierge/internal/escopo"
)

// Autenticador diz quem fez a requisição.
type Autenticador interface {
	Autenticar(r *http.Request) (usuarioID string, err error)
}

const (
	cabecalhoUsuarioDev = "X-Dev-Usuario"
	// usuarioSemeadoDev é o admin da Superlógica que a migration semeia para desenvolvimento,
	// o padrão de AUTH_DEV_USUARIO.
	usuarioSemeadoDev = "admin-superlogica"
)

// usuariosSemeadosDev são os usuários que as migrations semeiam para desenvolvimento: fora de
// AUTH_MODE=dev ninguém entra com eles, venha de que autenticador vier. Só lido, nunca escrito.
var usuariosSemeadosDev = map[string]bool{
	usuarioSemeadoDev: true, "construtor-superlogica": true, "gestor-exemplo": true, "gestor-avancado": true,
}

func semeadoDev(id string) bool { return usuariosSemeadosDev[id] }

var (
	errSemUsuario      = errors.New("requisição sem usuário")
	errSemAutenticacao = errors.New("AUTH_MODE sem autenticação disponível")
)

// autenticadorDev confia no cabeçalho X-Dev-Usuario; sem ele, usa o padrão (AUTH_DEV_USUARIO).
// validarConfig impede este modo em produção.
type autenticadorDev struct{ padrao string }

func (a autenticadorDev) Autenticar(r *http.Request) (string, error) {
	if u := strings.TrimSpace(r.Header.Get(cabecalhoUsuarioDev)); u != "" {
		return u, nil
	}
	if a.padrao != "" {
		return a.padrao, nil
	}
	return "", errSemUsuario
}

// autenticadorFechado recusa todos: fora de AUTH_MODE=dev ainda não há login, e a API fica
// fechada (o webhook do ATENDE e o healthz não passam por aqui).
type autenticadorFechado struct{}

func (autenticadorFechado) Autenticar(*http.Request) (string, error) { return "", errSemAutenticacao }

func novoAutenticador(authMode, usuarioPadrao string) Autenticador {
	if authMode == authModeDev {
		return autenticadorDev{padrao: strings.TrimSpace(usuarioPadrao)}
	}
	return autenticadorFechado{}
}

// carregadorAcesso é o acesso.Postgres; os testes usam um fake.
type carregadorAcesso interface {
	Carregar(ctx context.Context, usuarioID string, imp escopo.Implantacao) (acesso.Acesso, error)
}

type chaveAcesso struct{}

func acessoDe(ctx context.Context) (acesso.Acesso, bool) {
	a, ok := ctx.Value(chaveAcesso{}).(acesso.Acesso)
	return a, ok
}

// autenticado identifica o usuário, carrega o Acesso dele e o põe no contexto. Falha de
// identificação e usuário desconhecido são 401; erro ao carregar é 500 sem detalhe.
func (s *servidor) autenticado(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		usuarioID, err := s.autenticador.Autenticar(r)
		if err != nil || (semeadoDev(usuarioID) && s.authMode != authModeDev) {
			s.recusarLogin(w)
			return
		}
		anotarUsuario(r.Context(), usuarioID)
		a, err := s.acessos.Carregar(r.Context(), usuarioID, s.implantacao)
		if errors.Is(err, acesso.ErrUsuarioDesconhecido) {
			s.recusarLogin(w)
			return
		}
		if err != nil {
			s.responderErroInterno(w, r, "carregar acesso", err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), chaveAcesso{}, a)))
	})
}

// naoAutenticado é o corpo do 401. Dev diz ao Studio que pode oferecer o "Ver como" mesmo sem
// /api/eu; fora de AUTH_MODE=dev o campo nem aparece.
type naoAutenticado struct {
	Codigo string `json:"codigo"`
	Error  string `json:"error"`
	Dev    bool   `json:"dev,omitempty"`
}

func responderNaoAutenticado(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, naoAutenticado{Codigo: "NAO_AUTENTICADO", Error: "usuário não autenticado"})
}

// recusarLogin é o 401 do middleware: o único ponto em que o usuário ainda não foi reconhecido.
func (s *servidor) recusarLogin(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, naoAutenticado{Codigo: "NAO_AUTENTICADO", Error: "usuário não autenticado", Dev: s.authMode == authModeDev})
}

func responderSemPermissao(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, map[string]string{"codigo": "SEM_PERMISSAO", "error": "sem permissão para esta operação"})
}

// responderErroInterno: o erro vai só para o log; o cliente recebe uma mensagem fixa.
func (s *servidor) responderErroInterno(w http.ResponseWriter, r *http.Request, operacao string, err error) {
	s.logger().ErrorContext(r.Context(), "erro interno", "operacao", operacao, "erro", err.Error())
	writeJSON(w, http.StatusInternalServerError, map[string]string{"codigo": "ERRO_INTERNO", "error": "erro interno; tente de novo"})
}
