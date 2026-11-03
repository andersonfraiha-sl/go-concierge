// okf-admin: o servidor do concierge em desenvolvimento. Serve o Admin (cadastro de
// skills em pasta, com vínculos e declarações de recursos do Nexus), o Console
// (emulador do ATENDE) e o webhook do ATENDE.
//
//	okf-admin -root ./okf -addr 127.0.0.1:8090
//	okf-admin importar-json -dir okf/var   (leva os JSON antigos para o Postgres)
//
// Os recursos do Nexus vêm de NEXUS_BASE_URL (GET /v1/accounts/{account}/resources) ou,
// sem Nexus, do instantâneo okf/nexus/<account>.yaml. DEFAULT_ACCOUNT_ID é a account desse
// catálogo e do condomínio de exemplo de dev; o turno usa a account do condomínio resolvido.
// A identidade do morador vem do Gruvi com o token B2C configurado para o MCP.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go-concierge/internal/acesso"
	"go-concierge/internal/admin"
	"go-concierge/internal/atende"
	"go-concierge/internal/banco"
	"go-concierge/internal/env"
	"go-concierge/internal/escopo"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/mcp"
	"go-concierge/internal/memoria"
	"go-concierge/internal/nexus"
	"go-concierge/internal/okf"
	"go-concierge/internal/pedidos"
	"go-concierge/web"
)

type servidor struct {
	cat     *okf.Catalogo
	web     fs.FS
	account string
	nexus   string // origem dos recursos: url do Nexus, "instantâneo" ou "nenhuma"
	log     *log.Logger

	// canal ATENDE (webhook + emulador no Console)
	atende              *atende.Servico
	outbox              *atende.Outbox
	gruvi               *gruvi.Client
	gruviOK             bool
	gruviBFF            string
	gruviCompany        string
	webhookToken        string
	atendeAPISecret     string
	replyURL            string
	replyEmulada        bool
	atendeBaseURL       string
	atendeAPIKey        string
	atendeTenant        string
	atendeTelefoneCanal string
	telefoneMockB2C     string // só com AMBIENTE=dev: todo evento real vira este telefone (vazio = desligado)
	emulador            *atende.Emulador
	provedor            string
	memorias            memoria.Store
	admin               admin.Repositorio
	implantacao         escopo.Implantacao // ADMINISTRADORA_ID: o processo atende só essa (vazia = todas)
	banco               banco.Banco

	// acesso: quem é (autenticador) e o que pode (acessos); registro é o log JSON por requisição
	autenticador Autenticador
	acessos      carregadorAcesso
	registro     *slog.Logger
	authMode     string // AUTH_MODE normalizado: o usuário semeado de dev só vale com "dev"
}

const (
	timeoutBoot    = 30 * time.Second
	timeoutHealthz = time.Second
)

var (
	errUnauthorized  = errors.New("não autorizado")
	errNaoEncontrado = errors.New("protocolo não encontrado")
	errTelefone      = errors.New("informe phone")
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == subcomandoImportarJSON {
		os.Exit(importarJSON(os.Args[2:], os.Stdout, os.Stderr))
	}
	root := flag.String("root", "okf", "raiz do catálogo (agentes/, nexus/)")
	addr := flag.String("addr", "127.0.0.1:8090", "endereço HTTP")
	abrir := flag.Bool("open", false, "abre o navegador")
	webDir := flag.String("web", "", "serve a interface desta pasta em vez da embutida (desenvolvimento: web/okf)")
	dotenv := flag.String("env", ".env", "arquivo NOME=valor (LLM_MODE e chaves, NEXUS_*, DEFAULT_ACCOUNT_ID, GRUVI_*, ATENDE_*)")
	migrar := flag.Bool("migrar", false, "aplica as migrations no boot (padrão: ligado com AMBIENTE vazio ou dev, lido também do -env)")
	flag.Parse()
	env.Carregar(*dotenv)
	registro := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(registro) // o log.Printf legado passa a sair pelo mesmo handler JSON
	cfg := normalizarConfig(Config{Ambiente: os.Getenv("AMBIENTE"), AuthMode: os.Getenv("AUTH_MODE"), AtendeSecret: os.Getenv("ATENDE_API_SECRET")})
	if err := validarConfig(cfg); err != nil {
		log.Fatal(err)
	}
	deveMigrar := migrarPorPadrao(cfg.Ambiente)
	if flagInformada("migrar") {
		deveMigrar = *migrar
	}

	db, err := abrirBanco(os.Getenv("DATABASE_URL"), deveMigrar)
	if err != nil {
		log.Fatal(err)
	}

	cat, err := okf.NewCatalogo(*root)
	if err != nil {
		log.Fatal(err)
	}
	account := env.Ou("DEFAULT_ACCOUNT_ID", "cond-001")
	cli := nexus.New(os.Getenv("NEXUS_BASE_URL"), os.Getenv("NEXUS_TOKEN"))
	// Recursos: o catálogo MCP (tools/list) tem precedência; depois o Nexus ao vivo ou o
	// instantâneo nexus/<account>.yaml. Sem nenhum, o catálogo não confere declarações.
	mcpCli, mcpModo, err := mcp.FromEnv(os.Getenv)
	if err != nil {
		log.Fatal(err)
	}
	fonteNexus := nexus.Fonte(cli, account, filepath.Join(cat.Root, "nexus"))
	cat.Recursos = mcp.Combinar(mcp.Fonte(mcpCli, time.Minute, log.Printf), fonteNexus)
	s := &servidor{cat: cat, web: web.OKF, account: account, nexus: "nenhuma", log: log.Default(), banco: db,
		implantacao:  escopo.Implantacao{AdministradoraFixa: strings.TrimSpace(os.Getenv("ADMINISTRADORA_ID"))},
		autenticador: novoAutenticador(cfg.AuthMode, os.Getenv("AUTH_DEV_USUARIO")),
		authMode:     cfg.AuthMode,
		acessos:      acesso.NovoPostgres(db.Ent),
		registro:     registro,
	}
	var origens []string
	if mcpCli != nil {
		origens = append(origens, "MCP "+mcpCli.URL+" ("+mcpModo+")")
	}
	switch {
	case cli.BaseURL != "":
		origens = append(origens, cli.BaseURL)
	case fonteNexus != nil:
		origens = append(origens, "instantâneo nexus/"+account+".yaml")
	}
	if len(origens) > 0 {
		s.nexus = strings.Join(origens, " + ")
	}
	if *webDir != "" {
		s.web = os.DirFS(*webDir)
	}

	// canal ATENDE: identidade pelo Gruvi, concierge pelo harness, resposta pela caixa de saída
	var tokenGruvi gruvi.TokenProvider
	if mcpCli != nil && mcpModo == "b2c" {
		tokenGruvi = mcpCli.Token
	}
	s.gruvi = gruvi.New(os.Getenv("GRUVI_BFF_URL"), os.Getenv("GRUVI_COMPANY_ID"), os.Getenv("MCP_B2C_MOBILE"), tokenGruvi)
	s.gruviOK, s.gruviBFF, s.gruviCompany = s.gruvi.Configurado(), s.gruvi.BFFURL, s.gruvi.CompanyID
	s.webhookToken = os.Getenv("ATENDE_WEBHOOK_TOKEN")
	s.atendeAPISecret = os.Getenv("ATENDE_API_SECRET")
	s.replyURL = os.Getenv("ATENDE_REPLY_URL")
	s.atendeBaseURL = os.Getenv("ATENDE_API_BASE_URL")
	s.atendeAPIKey = os.Getenv("ATENDE_API_KEY")
	s.atendeTenant = os.Getenv("ATENDE_TENANT_ID")
	s.atendeTelefoneCanal = os.Getenv("ATENDE_TENANT_TELEFONE")
	replyURLLegada := os.Getenv("ATENDE_REPLY_URL")
	s.outbox = &atende.Outbox{}
	s.emulador = atende.NewEmulador()
	s.replyURL = "http://" + *addr + "/emulador/atende/messages"
	s.replyEmulada = true
	s.outbox.Emulado = &atende.HTTPSender{URL: s.replyURL, Token: os.Getenv("ATENDE_REPLY_TOKEN")}
	if s.atendeBaseURL != "" && s.atendeAPIKey != "" {
		s.outbox.ATENDE = &atende.ATENDESender{BaseURL: s.atendeBaseURL, APIKey: s.atendeAPIKey}
	} else if replyURLLegada != "" {
		s.outbox.ATENDE = &atende.HTTPSender{URL: replyURLLegada, Token: os.Getenv("ATENDE_REPLY_TOKEN")}
	}
	s.provedor = harness.ResolverProvedor(os.Getenv).Nome
	cfgHarness := harness.Config{Nexus: cli, Env: os.Getenv}
	if mcpCli != nil {
		cfgHarness.MCP = mcpCli
	}
	// o Studio e o turno dividem o cache: as escritas de um aparecem no outro
	s.memorias = memoria.NovoCache(memoria.NovoPostgres(db))
	s.atende = &atende.Servico{
		Cat: cat, Agente: env.Ou("ATENDE_AGENTE", "agentes/concierge"), Implantacao: s.implantacao,
		Config: cfgHarness,
		Gruvi:  s.gruvi, Sender: s.outbox, Sessoes: atende.NewSessoes(atende.NovoSessaoPostgres(db)), Timeout: 120 * time.Second, Log: s.log,
		Memorias: s.memorias, Extrator: harness.NovoExtrator(context.Background(), cfgHarness),
		Pool: db.Pool,
	}
	if mcpModo == "b2c" {
		s.atende.TelefoneB2C = os.Getenv("MCP_B2C_MOBILE")
	}
	s.telefoneMockB2C = telefoneMockB2C(cfg.Ambiente, s.atende.TelefoneB2C)
	if s.telefoneMockB2C != "" {
		slog.Warn("webhook: mock B2C ligado, todo morador do ATENDE é tratado como o telefone de teste",
			"operacao", "mock_telefone_b2c", "ambiente", cfg.Ambiente, "telefone", escopo.MascararTelefone(s.telefoneMockB2C))
	}
	// registro de pedidos por contato (PEDIDOS=off desliga: a conversa segue só pelo orquestrador)
	if strings.ToLower(strings.TrimSpace(os.Getenv("PEDIDOS"))) != "off" {
		s.atende.Pedidos = pedidos.NovoCache(pedidos.NovoPostgres(db))
		s.atende.ExtratorPedidos = harness.NovoExtratorPedidos(context.Background(), cfgHarness)
	}

	// Admin das administradoras: mapeamento condomínio × casos de uso × payloads. O
	// condomínio de exemplo (DEFAULT_ACCOUNT_ID) só existe para o emulador em dev. Sem
	// GRUVI_COMPANY_ID ele ganha a company "exemplo": condomínio sem company não é atendido,
	// e sem Gruvi configurado a identidade não é consultada.
	adminPG := admin.NovoPostgres(db.Ent)
	s.admin = adminPG
	exemplo := admin.Condominio{ID: account, Nome: "Condomínio exemplo", Administradora: "adm-exemplo", Account: account, CompanyID: cmp.Or(s.gruviCompany, "exemplo"), Tenant: env.Ou("ATENDE_TENANT_ID", env.Ou("ATENDE_TENANT", "69a5a131e67bd8e2f54e5981"))}
	if err := semearMapeamento(s.admin, cfg.Ambiente, exemplo); err != nil {
		log.Fatal(err)
	}
	// o turno lê o mapeamento do cache (cada escrita do Admin o atualiza); Admin e RBAC leem
	// direto do banco. Aquecido aqui para nenhum turno pagar a primeira leitura.
	mapeamento := admin.NovoCache(adminPG)
	if _, err := mapeamento.Ler(context.Background()); err != nil {
		log.Printf("okf-admin: mapeamento não aquecido no boot, o primeiro turno lê do banco: %v", err)
	}
	s.atende.Mapeamento = mapeamento

	// O servidor MCP só escuta em loopback (na rede deste contêiner): /mcp/ aqui o repassa,
	// para testar de fora (Inspector, curl) pela porta do concierge. O token NÃO é injetado:
	// o cliente manda o seu Authorization.
	var proxy http.Handler
	if mcpCli != nil {
		p, base, err := proxyMCP(mcpCli)
		if err != nil {
			log.Printf("okf-admin: proxy do MCP desligado: %v", err)
		} else {
			proxy = p
			log.Printf("okf-admin: MCP exposto em http://%s%s (repassa a %s; Authorization do cliente)", *addr, mcpPath(mcpCli.URL), base)
		}
	}
	handler := s.rotear(proxy)

	log.Printf("okf-admin: catálogo %s · account %s · recursos: %s · modelo: %s · gruvi: %v · http://%s", cat.Root, account, s.nexus, s.provedor, s.gruviOK, *addr)
	if *abrir {
		go func() {
			time.Sleep(300 * time.Millisecond)
			abrirNavegador("http://" + *addr)
		}()
	}
	log.Fatal(http.ListenAndServe(*addr, handler))
}

// migrarPorPadrao liga as migrations no boot só em desenvolvimento; nos demais ambientes
// elas rodam como job antes do deploy.
func migrarPorPadrao(ambiente string) bool { return ehDesenvolvimento(ambiente) }

// ehDesenvolvimento: AMBIENTE vazio ou dev.
func ehDesenvolvimento(ambiente string) bool { return ambiente == "" || ambiente == "dev" }

// flagInformada diz se a flag veio na linha de comando (e não do padrão).
func flagInformada(nome string) bool {
	informada := false
	flag.Visit(func(f *flag.Flag) { informada = informada || f.Name == nome })
	return informada
}

const comoSubirPostgres = "suba o Postgres com ./build.sh --run, ou com docker compose up -d postgres e " +
	"DATABASE_URL=postgres://concierge:concierge@127.0.0.1:5432/concierge?sslmode=disable"

func abrirBanco(url string, migrar bool) (banco.Banco, error) {
	if url == "" {
		return banco.Banco{}, errors.New("DATABASE_URL vazia: " + comoSubirPostgres)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeoutBoot)
	defer cancel()
	db, err := banco.Abrir(ctx, url)
	if err != nil {
		return banco.Banco{}, fmt.Errorf("%w (%s)", err, comoSubirPostgres)
	}
	if !migrar {
		return db, nil
	}
	if err := banco.Migrar(ctx, db); err != nil {
		return banco.Banco{}, errors.Join(err, db.Fechar())
	}
	return db, nil
}

// semearMapeamento: em desenvolvimento, um banco sem mapeamento ganha a administradora de
// exemplo, dona do condomínio de exemplo (ligado à account, ao Gruvi e ao tenant do ATENDE).
// Fora de dev o boot não grava nada: mapeamento vazio só gera um WARN pedindo o importar-json.
func semearMapeamento(repo admin.Repositorio, ambiente string, exemplo admin.Condominio) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeoutBoot)
	defer cancel()
	m, err := repo.Ler(ctx)
	if err != nil {
		return fmt.Errorf("semear mapeamento: %w", err)
	}
	if len(m.Condominios) > 0 || len(m.Administradoras) > 0 {
		return nil
	}
	if !ehDesenvolvimento(ambiente) {
		slog.WarnContext(ctx, "mapeamento das administradoras vazio; rode okf-admin importar-json",
			"operacao", "semear_mapeamento", "ambiente", ambiente)
		return nil
	}
	err = gravarExemplo(ctx, repo, exemplo)
	if errors.Is(err, admin.ErrInvalido) {
		slog.WarnContext(ctx, "mapeamento de exemplo não semeado", "operacao", "semear_mapeamento", "erro", err)
		return nil
	}
	if err != nil {
		return fmt.Errorf("semear mapeamento: %w", err)
	}
	return nil
}

// gravarExemplo grava o condomínio primeiro, ainda sem dona: um DEFAULT_ACCOUNT_ID fora da
// regra de id é recusado antes de qualquer outra escrita.
func gravarExemplo(ctx context.Context, repo admin.Repositorio, exemplo admin.Condominio) error {
	semDona := exemplo
	semDona.Administradora = ""
	if _, err := repo.SalvarCondominio(ctx, semDona); err != nil {
		return err
	}
	if _, err := repo.SalvarAdministradora(ctx, admin.Administradora{ID: exemplo.Administradora, Nome: "Administradora exemplo"}); err != nil {
		return err
	}
	_, err := repo.SalvarCondominio(ctx, exemplo)
	return err
}

// healthz responde 503 quando o Postgres não responde em timeoutHealthz.
func (s *servidor) healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), timeoutHealthz)
	defer cancel()
	if err := s.banco.Pool.Ping(ctx); err != nil {
		s.log.Printf("okf-admin: healthz: ping no postgres: %v", err)
		http.Error(w, "postgres indisponível", http.StatusServiceUnavailable)
		return
	}
	w.Write([]byte("ok"))
}

// proxyMCP repassa /mcp/... ao servidor MCP configurado (MCP_URL), com o Host que ele
// exige. Só o caminho e o corpo passam; a autenticação é do cliente.
func proxyMCP(c *mcp.Client) (http.Handler, string, error) {
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" {
		return nil, "", fmt.Errorf("MCP_URL inválida: %q", c.URL)
	}
	alvo := &url.URL{Scheme: u.Scheme, Host: u.Host}
	host := c.HostHeader
	if host == "" {
		host = u.Host
	}
	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(alvo)
			pr.Out.Host = host
		},
		FlushInterval: -1, // Streamable HTTP responde em SSE
	}
	return rp, alvo.String(), nil
}

func mcpPath(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Path != "" {
		return u.Path
	}
	return "/mcp/"
}

func (s *servidor) rotas(mux *http.ServeMux) {
	mux.Handle(padraoEstaticos, semCache(http.FileServerFS(s.web)))
	mux.HandleFunc("GET /healthz", s.healthz)

	// leitura do catálogo: skills.ler em qualquer escopo
	for padrao, h := range map[string]http.HandlerFunc{
		"GET /api/anatomia":         s.anatomia,
		"GET /api/agentes":          s.arvore,
		"GET /api/agente":           s.agente,
		"GET /api/agentes/exportar": s.exportar,
		"GET /api/arquivo":          s.lerArquivo,
		"GET /api/arquivo/modelo":   s.modelo,
		"GET /api/projecao":         s.projecao,
		"GET /api/nexus/recursos":   s.recursos,
	} {
		mux.Handle(padrao, s.protegidaEmAlgum(acesso.SkillsLer, h))
	}
	// fluxo dos agentes: skills.editar_fluxo na Superlógica
	for padrao, h := range map[string]http.HandlerFunc{
		"POST /api/agentes":          s.criar,
		"PUT /api/agente/config":     s.config,
		"DELETE /api/agente":         s.excluir,
		"POST /api/agentes/mover":    s.mover,
		"POST /api/agentes/importar": s.importar,
		"PUT /api/arquivo":           s.salvarArquivo,
		"DELETE /api/arquivo":        s.excluirArquivo,
	} {
		mux.Handle(padrao, s.protegida(acesso.SkillsEditarFluxo, alvoSuperlogica, h))
	}
	// recursos do Nexus: ferramentas.editar na Superlógica
	for padrao, h := range map[string]http.HandlerFunc{
		"PUT /api/nexus/recurso":    s.salvarRecurso,
		"DELETE /api/nexus/recurso": s.removerRecurso,
		"POST /api/agente/recursos": s.vincular,
	} {
		mux.Handle(padrao, s.protegida(acesso.FerramentasEditar, alvoSuperlogica, h))
	}
}

// ---- handlers ------------------------------------------------------------

func (s *servidor) anatomia(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"caixinhas":    okf.Caixinhas,
		"tipos_okf":    okf.TiposOKF,
		"raiz":         []string{okf.AgentYAML, okf.PersonaMD, okf.CasoYAML},
		"account":      s.account,
		"nexus_origem": s.nexus,
	})
}

func (s *servidor) arvore(w http.ResponseWriter, _ *http.Request) {
	tree, err := s.cat.Tree()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"agentes": resumir(tree)})
}

type no struct {
	Caminho    string `json:"caminho"`
	Nome       string `json:"nome"`
	Tipo       string `json:"tipo"`
	Descricao  string `json:"descricao"`
	Caso       bool   `json:"caso"`
	Erros      int    `json:"erros"`
	Avisos     int    `json:"avisos"`
	Subagentes []no   `json:"subagentes"`
}

func resumir(tree []*okf.Agent) []no {
	out := []no{}
	for _, a := range tree {
		n := no{Caminho: a.Rel, Nome: a.Nome, Tipo: a.Tipo, Descricao: a.Descricao, Caso: a.Caso != nil, Subagentes: resumir(a.Subagentes)}
		for _, p := range a.Problemas {
			if p.Nivel == "erro" {
				n.Erros++
			} else {
				n.Avisos++
			}
		}
		out = append(out, n)
	}
	return out
}

func (s *servidor) carregar(rel string) (*okf.Agent, error) {
	dir, err := s.cat.ResolveAgente(rel)
	if err != nil {
		return nil, err
	}
	if !okf.IsAgent(dir) {
		return nil, fmt.Errorf("agente não existe: %s", rel)
	}
	return s.cat.Load(dir)
}

func (s *servidor) agente(w http.ResponseWriter, r *http.Request) {
	a, err := s.carregar(r.URL.Query().Get("agente"))
	if err != nil {
		writeErr(w, 404, err)
		return
	}
	writeJSON(w, 200, map[string]any{
		"agente":        a,
		"vinculos":      s.cat.Vinculos(a),
		"recursos":      s.cat.RecursosDoAgente(a),
		"conhecimento":  s.cat.Conhecimento(a),
		"indice":        s.cat.Indice(a),
		"procedimentos": s.cat.Procedimentos(a),
		"workflow":      workflowPrincipal(a),
		"pai":           paiRel(a.Rel),
	})
}

// workflowPrincipal devolve o texto de workflows/principal.md (sem frontmatter), que a
// ficha mostra como "a ordem do acontecimento" do agente.
func workflowPrincipal(a *okf.Agent) string {
	raw, err := os.ReadFile(filepath.Join(a.Path, "workflows", "principal.md"))
	if err != nil {
		return ""
	}
	return okf.StripFrontmatter(string(raw))
}

func paiRel(rel string) string {
	i := strings.LastIndex(rel, "/"+okf.DirAgentes+"/")
	if i == -1 {
		return ""
	}
	return rel[:i]
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

func (s *servidor) config(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Agente string         `json:"agente"`
		Campos map[string]any `json:"campos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.cat.SalvarConfig(corpo.Agente, corpo.Campos); err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"salvo": corpo.Agente})
}

func (s *servidor) excluir(w http.ResponseWriter, r *http.Request) {
	mud, err := s.cat.ExcluirAgente(r.URL.Query().Get("agente"))
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"excluido": r.URL.Query().Get("agente"), "mudancas": mud})
}

func (s *servidor) mover(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Origem  string `json:"origem"`
		Destino string `json:"destino"`
		Clonar  bool   `json:"clonar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	rel, mud, err := s.cat.Mover(corpo.Origem, corpo.Destino, corpo.Clonar)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"caminho": rel, "mudancas": mud})
}

func (s *servidor) importar(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("caminho")
	dados, err := io.ReadAll(io.LimitReader(r.Body, 100<<20))
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	arqs, err := s.cat.ImportarZip(rel, dados)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"instalado": rel, "arquivos": arqs})
}

func (s *servidor) exportar(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("agente")
	dados, err := s.cat.ExportarZip(rel)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(rel)+".zip"))
	w.Write(dados)
}

func (s *servidor) lerArquivo(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	raw, err := s.cat.LerArquivo(q.Get("agente"), q.Get("caminho"))
	if err != nil {
		writeErr(w, 404, err)
		return
	}
	writeJSON(w, 200, map[string]any{"caminho": q.Get("caminho"), "conteudo": string(raw)})
}

func (s *servidor) salvarArquivo(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Agente   string `json:"agente"`
		Caminho  string `json:"caminho"`
		Conteudo string `json:"conteudo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.cat.SalvarArquivo(corpo.Agente, corpo.Caminho, []byte(corpo.Conteudo)); err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"salvo": corpo.Caminho})
}

func (s *servidor) excluirArquivo(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if err := s.cat.ExcluirArquivo(q.Get("agente"), q.Get("caminho")); err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"excluido": q.Get("caminho")})
}

func (s *servidor) modelo(w http.ResponseWriter, r *http.Request) {
	rel := r.URL.Query().Get("caminho")
	writeJSON(w, 200, map[string]any{"caminho": rel, "conteudo": okf.ModeloDeArquivo(rel)})
}

func (s *servidor) projecao(w http.ResponseWriter, r *http.Request) {
	a, err := s.carregar(r.URL.Query().Get("agente"))
	if err != nil {
		writeErr(w, 404, err)
		return
	}
	writeJSON(w, 200, map[string]any{"projecao": s.cat.Projecao(a), "roteador": s.cat.Roteador(a)})
}

func (s *servidor) recursos(w http.ResponseWriter, _ *http.Request) {
	lista := s.cat.RecursosNexus()
	if lista == nil {
		lista = []okf.Recurso{}
	}
	writeJSON(w, 200, map[string]any{"account": s.account, "origem": s.nexus, "conferido": s.cat.TemNexus(), "recursos": lista})
}

// instantaneoEditavel: o catálogo só se edita aqui quando vem do instantâneo YAML; ao
// vivo, o dono é o Nexus.
func (s *servidor) instantaneoEditavel() (string, error) {
	if strings.HasPrefix(s.nexus, "http") || strings.Contains(s.nexus, "+ http") {
		return "", errors.New("catálogo ao vivo do Nexus: edite o recurso lá")
	}
	return filepath.Join(s.cat.Root, "nexus", s.account+".yaml"), nil
}

func (s *servidor) salvarRecurso(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Original string      `json:"original"`
		Recurso  okf.Recurso `json:"recurso"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	p, err := s.instantaneoEditavel()
	if err != nil {
		writeErr(w, 409, err)
		return
	}
	if err := okf.SalvarRecursoFixture(p, corpo.Original, corpo.Recurso); err != nil {
		writeErr(w, 400, err)
		return
	}
	s.recursos(w, r)
}

func (s *servidor) removerRecurso(w http.ResponseWriter, r *http.Request) {
	nome := r.URL.Query().Get("nome")
	p, err := s.instantaneoEditavel()
	if err != nil {
		writeErr(w, 409, err)
		return
	}
	if err := okf.RemoverRecursoFixture(p, nome); err != nil {
		writeErr(w, 400, err)
		return
	}
	s.recursos(w, r)
}

func (s *servidor) vincular(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Agente string `json:"agente"`
		Nome   string `json:"nome"`
		Ligar  bool   `json:"ligar"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	mud, err := s.cat.VincularRecurso(corpo.Agente, corpo.Nome, corpo.Ligar)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if mud == nil {
		mud = []string{"nada a fazer"}
	}
	writeJSON(w, 200, map[string]any{"recurso": corpo.Nome, "ligado": corpo.Ligar, "mudancas": mud})
}

// ---- util ----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	if err == nil {
		err = errors.New("erro")
	}
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

func semCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}

func abrirNavegador(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	_ = cmd.Start()
}
