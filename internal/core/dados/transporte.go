package dados

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"go-concierge/internal/core/administracao/regras"
	"go-concierge/internal/escopo"
	atende "go-concierge/internal/harness/atendimento"
	"go-concierge/internal/harness/canais"
	admin "go-concierge/internal/harness/configuracao"
	"go-concierge/internal/harness/ganchos/auditoria"
	"go-concierge/internal/harness/pedidos"
)

const limiteTurno = 64 << 20
const rotaTurno = "/interno/dados/v1/turnos"
const prazoGravacaoTurno = 3 * time.Second

type comandoTurno struct {
	Operacao  string                `json:"operacao"`
	Sessao    atende.RegistroSessao `json:"sessao"`
	Contato   escopo.Contato        `json:"contato"`
	Carteira  pedidos.Carteira      `json:"carteira"`
	Auditoria []auditoria.Linha     `json:"auditoria"`
}

type respostaTurno struct {
	Operacao  string `json:"operacao"`
	Resultado string `json:"resultado"`
}

type Servidor struct {
	http       *http.Server
	cliente    *Cliente
	fim        chan struct{}
	manutencao *Manutencao
	saida      *CanaisDeSaida
	catalogo   *CatalogoHTTP
}

type Cliente struct {
	base            string
	url             string
	token           string
	http            *http.Client
	regrasMu        sync.Mutex
	aoAlterarRegras func([]regras.RegraGravada)
}

type Config struct {
	Turno         atende.GravacaoTurno
	Canais        canais.Repositorio
	Sessoes       atende.SessaoStore
	Memorias      OperacoesMemoria
	Pedidos       OperacoesPedidos
	Auditoria     OperacoesAuditoria
	Acessos       *AcessosPrivados
	Conteudo      LeitorConteudo
	Regras        regras.FonteDoIndice
	Manutencao    OperacoesManutencao
	Configuracao  LeitorConfiguracao
	Catalogo      RepositorioCatalogo
	Administracao ConsultaAdministracao
	Consultas     *ConsultasPostgres
}

func IniciarConfigurado(config Config) (*Servidor, *Cliente, *Entrada, error) {
	if config.Conteudo != nil && config.Canais == nil {
		return nil, nil, nil, errors.New("conteúdo exige repositório de canais")
	}
	return iniciar(config)
}

func Iniciar(gravador atende.GravacaoTurno) (*Servidor, *Cliente, error) {
	servidor, cliente, _, err := iniciar(Config{Turno: gravador})
	return servidor, cliente, err
}

func IniciarComCanais(gravador atende.GravacaoTurno, repo canais.Repositorio, stores ...atende.SessaoStore) (*Servidor, *Cliente, *Entrada, error) {
	if repo == nil {
		return nil, nil, nil, errors.New("repositório de canais ausente")
	}
	if len(stores) > 1 || len(stores) == 1 && stores[0] == nil {
		return nil, nil, nil, errors.New("repositório de sessões inválido")
	}
	var sessoes atende.SessaoStore
	if len(stores) == 1 {
		sessoes = stores[0]
	}
	return iniciar(Config{Turno: gravador, Canais: repo, Sessoes: sessoes})
}

func IniciarComDados(gravador atende.GravacaoTurno, repo canais.Repositorio, sessoes atende.SessaoStore, memorias OperacoesMemoria, pedidos ...OperacoesPedidos) (*Servidor, *Cliente, *Entrada, error) {
	if repo == nil || sessoes == nil || memorias == nil {
		return nil, nil, nil, errors.New("repositórios de dados incompletos")
	}
	if len(pedidos) > 1 || len(pedidos) == 1 && pedidos[0] == nil {
		return nil, nil, nil, errors.New("repositório de pedidos inválido")
	}
	var storePedidos OperacoesPedidos
	if len(pedidos) == 1 {
		storePedidos = pedidos[0]
	}
	return iniciar(Config{Turno: gravador, Canais: repo, Sessoes: sessoes, Memorias: memorias, Pedidos: storePedidos})
}

func IniciarCompleto(gravador atende.GravacaoTurno, repo canais.Repositorio, sessoes atende.SessaoStore, memorias OperacoesMemoria, pedidos OperacoesPedidos, auditoria OperacoesAuditoria, acessos ...*AcessosPrivados) (*Servidor, *Cliente, *Entrada, error) {
	if repo == nil || sessoes == nil || memorias == nil || pedidos == nil || auditoria == nil {
		return nil, nil, nil, errors.New("repositórios de dados incompletos")
	}
	if len(acessos) > 1 || len(acessos) == 1 && (acessos[0] == nil || acessos[0].verificador == nil || acessos[0].store == nil) {
		return nil, nil, nil, errors.New("repositório de acessos inválido")
	}
	var acesso *AcessosPrivados
	if len(acessos) == 1 {
		acesso = acessos[0]
	}
	return iniciar(Config{Turno: gravador, Canais: repo, Sessoes: sessoes, Memorias: memorias, Pedidos: pedidos, Auditoria: auditoria, Acessos: acesso})
}

func iniciar(config Config) (*Servidor, *Cliente, *Entrada, error) {
	if config.Acessos != nil && (config.Acessos.verificador == nil || config.Acessos.store == nil) {
		return nil, nil, nil, errors.New("repositório de acessos inválido")
	}
	gravador, repo, sessoes := config.Turno, config.Canais, config.Sessoes
	memorias, pedidos, auditoria, acessos := config.Memorias, config.Pedidos, config.Auditoria, config.Acessos
	if gravador == nil {
		return nil, nil, nil, errors.New("gravador de turno ausente")
	}
	segredo := make([]byte, 32)
	if _, err := rand.Read(segredo); err != nil {
		return nil, nil, nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(segredo)
	if _, err := rand.Read(segredo); err != nil {
		return nil, nil, nil, err
	}
	tokenEntrada := base64.RawURLEncoding.EncodeToString(segredo)
	if _, err := rand.Read(segredo); err != nil {
		return nil, nil, nil, err
	}
	tokenManutencao := base64.RawURLEncoding.EncodeToString(segredo)
	if _, err := rand.Read(segredo); err != nil {
		return nil, nil, nil, err
	}
	tokenSaida := base64.RawURLEncoding.EncodeToString(segredo)
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, nil, nil, err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+rotaTurno, func(w http.ResponseWriter, r *http.Request) {
		if !hmac.Equal([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) {
			http.Error(w, "não autorizado", http.StatusUnauthorized)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limiteTurno)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		var cmd comandoTurno
		if err := dec.Decode(&cmd); err != nil {
			http.Error(w, "comando inválido", http.StatusBadRequest)
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			http.Error(w, "comando inválido", http.StatusBadRequest)
			return
		}
		if _, err := uuid.Parse(cmd.Operacao); err != nil {
			http.Error(w, "operação inválida", http.StatusBadRequest)
			return
		}
		err := gravador.Gravar(r.Context(), cmd.Sessao, cmd.Contato, cmd.Carteira, cmd.Auditoria)
		resultado, status := "aplicada", http.StatusOK
		if errors.Is(err, atende.ErrGravacaoRecusada) {
			resultado, status = "recusada", http.StatusForbidden
		} else if errors.Is(err, atende.ErrGravacaoNaoAplicada) {
			resultado, status = "nao_aplicada", http.StatusUnprocessableEntity
		} else if err != nil {
			resultado, status = "incerta", http.StatusInternalServerError
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(respostaTurno{Operacao: cmd.Operacao, Resultado: resultado})
	})
	if repo != nil {
		registrarCanais(mux, repo, token, tokenEntrada)
		registrarCanalDeSaida(mux, repo, tokenSaida)
	}
	if config.Conteudo != nil {
		registrarConteudo(mux, repo, config.Conteudo, token)
	}
	if config.Configuracao != nil && repo != nil {
		registrarConfiguracao(mux, repo, config.Configuracao, token)
	}
	if config.Regras != nil {
		registrarRegrasAtivas(mux, config.Regras, token)
	}
	if config.Manutencao != nil {
		registrarManutencao(mux, config.Manutencao, tokenManutencao)
	}
	if config.Catalogo != nil {
		registrarCatalogo(mux, config.Catalogo, tokenManutencao)
	}
	if sessoes != nil {
		registrarSessoes(mux, repo, sessoes, token)
	}
	if memorias != nil {
		registrarMemorias(mux, memorias, token)
	}
	if pedidos != nil {
		registrarPedidos(mux, pedidos, token)
	}
	if auditoria != nil {
		registrarAuditoria(mux, auditoria, token)
	}
	if acessos != nil {
		if config.Consultas != nil {
			registrarConsultasAdministrativas(mux, config.Consultas, acessos, token)
		}
		if regrasAdmin, ok := config.Regras.(RegrasAdministrativas); ok {
			registrarRegrasAdministrativas(mux, regrasAdmin, acessos, token)
		}
		if config.Administracao != nil {
			registrarConsultaAdministracao(mux, config.Administracao, acessos, token)
			if edicao, ok := config.Administracao.(admin.Repositorio); ok {
				registrarEdicaoAdministracao(mux, edicao, acessos, token)
			}
		}
		if admin, ok := auditoria.(auditoriaAdministrativa); ok {
			registrarAuditoriaAdmin(mux, admin, acessos, token)
		}
		registrarAcessos(mux, acessos, token)
		registrarContatosAdmin(mux, acessos, token)
		if sessoes != nil {
			registrarSessoesAdmin(mux, sessoes, acessos, token)
		}
	}
	transporte := &http.Transport{Proxy: nil}
	base := "http://" + listener.Addr().String()
	cliente := &Cliente{base: base, url: base + rotaTurno, token: token, http: &http.Client{Transport: transporte, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	if config.Regras != nil {
		config.Regras.ObservarAlteracoes(cliente.notificarRegras)
	}
	entrada := &Entrada{base: base, token: tokenEntrada, http: cliente.http}
	servidor := &Servidor{http: &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second}, cliente: cliente, fim: make(chan struct{})}
	if repo != nil {
		servidor.saida = &CanaisDeSaida{base: base, token: tokenSaida, http: cliente.http}
	}
	if config.Manutencao != nil {
		servidor.manutencao = &Manutencao{base: base, token: tokenManutencao, http: cliente.http}
	}
	if config.Catalogo != nil {
		servidor.catalogo = &CatalogoHTTP{base: base, token: tokenManutencao, http: cliente.http}
	}
	go func() {
		err := servidor.http.Serve(listener)
		close(servidor.fim)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("servidor de dados interrompido", "operacao", "servir_dados", "erro", err)
		}
	}()
	return servidor, cliente, entrada, nil
}

func (s *Servidor) Pronto() bool {
	select {
	case <-s.fim:
		return false
	default:
		return true
	}
}

func (s *Servidor) Fechar(ctx context.Context) error {
	s.cliente.http.CloseIdleConnections()
	return s.http.Shutdown(ctx)
}

func (c *Cliente) Gravar(ctx context.Context, r atende.RegistroSessao, contato escopo.Contato, cart pedidos.Carteira, linhas []auditoria.Linha) error {
	cart, err := pedidos.CarteiraParaTransporte(contato, cart)
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	contato = contato.Canonico()
	id, err := uuid.NewRandom()
	if err != nil {
		return errors.Join(atende.ErrGravacaoNaoAplicada, err)
	}
	cmd := comandoTurno{Operacao: id.String(), Sessao: r, Contato: contato, Carteira: cart, Auditoria: linhas}
	var resposta respostaTurno
	status, enviada, err := requisitarJSON(ctx, c.http, c.url, c.token, cmd, &resposta, limiteTurno, 4096, prazoGravacaoTurno)
	if err != nil {
		if !enviada {
			return errors.Join(atende.ErrGravacaoNaoAplicada, err)
		}
		return erroIncerto(id, err)
	}
	if resposta.Operacao != id.String() {
		return erroIncerto(id, errors.New("resposta inválida da operação de dados"))
	}
	if status == http.StatusOK && resposta.Resultado == "aplicada" {
		return nil
	}
	if status == http.StatusUnprocessableEntity && resposta.Resultado == "nao_aplicada" {
		return atende.ErrGravacaoNaoAplicada
	}
	if status == http.StatusForbidden && resposta.Resultado == "recusada" {
		return errors.Join(atende.ErrGravacaoNaoAplicada, atende.ErrGravacaoRecusada)
	}
	return erroIncerto(id, fmt.Errorf("HTTP %d", status))
}

func erroIncerto(id uuid.UUID, err error) error {
	return errors.Join(atende.ErrGravacaoIncerta, fmt.Errorf("operação %s: %w", id, err))
}
