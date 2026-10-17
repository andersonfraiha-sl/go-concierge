// okf-admin: o servidor do concierge em desenvolvimento. Serve o Admin (cadastro de
// skills em pasta, com vínculos e declarações de recursos do Nexus), o Console
// (emulador do ATENDE) e o webhook do ATENDE.
//
//	okf-admin -root ./okf -addr 127.0.0.1:8090
//
// Os recursos do Nexus vêm de NEXUS_BASE_URL (GET /v1/accounts/{account}/resources) ou,
// sem Nexus, do instantâneo okf/nexus/<account>.yaml. A account é DEFAULT_ACCOUNT_ID.
// A identidade do morador vem do Gruvi (GRUVI_CLIENT_BASIC, GRUVI_COMPANY_ID).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go-concierge/internal/atende"
	"go-concierge/internal/env"
	"go-concierge/internal/gruvi"
	"go-concierge/internal/harness"
	"go-concierge/internal/nexus"
	"go-concierge/internal/okf"
	"go-concierge/web"
)

type servidor struct {
	cat     *okf.Catalogo
	web     fs.FS
	account string
	nexus   string // origem dos recursos: url do Nexus, "instantâneo" ou "nenhuma"
	log     *log.Logger

	// canal ATENDE (webhook + emulador no Console)
	atende        *atende.Servico
	outbox        *atende.Outbox
	gruvi         *gruvi.Client
	gruviOK       bool
	gruviTokenURL string
	gruviBFF      string
	gruviCompany  string
	webhookToken  string
	replyURL      string
	provedor      string
}

var (
	errUnauthorized  = errors.New("não autorizado")
	errNaoEncontrado = errors.New("protocolo não encontrado")
	errTelefone      = errors.New("informe phone")
)

func main() {
	root := flag.String("root", "okf", "raiz do catálogo (agentes/, nexus/)")
	addr := flag.String("addr", "127.0.0.1:8090", "endereço HTTP")
	abrir := flag.Bool("open", false, "abre o navegador")
	webDir := flag.String("web", "", "serve a interface desta pasta em vez da embutida (desenvolvimento: web/okf)")
	dotenv := flag.String("env", ".env", "arquivo NOME=valor (LLM_MODE e chaves, NEXUS_*, DEFAULT_ACCOUNT_ID, GRUVI_*, ATENDE_*)")
	flag.Parse()
	env.Carregar(*dotenv)

	cat, err := okf.NewCatalogo(*root)
	if err != nil {
		log.Fatal(err)
	}
	account := env.Ou("DEFAULT_ACCOUNT_ID", "cond-001")
	cli := nexus.New(os.Getenv("NEXUS_BASE_URL"), os.Getenv("NEXUS_TOKEN"))
	cat.Recursos = nexus.Fonte(cli, account, filepath.Join(cat.Root, "nexus"))
	s := &servidor{cat: cat, web: web.OKF, account: account, nexus: "nenhuma", log: log.Default()}
	switch {
	case cli.BaseURL != "":
		s.nexus = cli.BaseURL
	case cat.Recursos != nil:
		s.nexus = "instantâneo nexus/" + account + ".yaml"
	}
	if *webDir != "" {
		s.web = os.DirFS(*webDir)
	}

	// canal ATENDE: identidade pelo Gruvi, concierge pelo harness, resposta pela caixa de saída
	s.gruvi = gruvi.New(os.Getenv("GRUVI_TOKEN_URL"), os.Getenv("GRUVI_BFF_URL"), os.Getenv("GRUVI_CLIENT_BASIC"), os.Getenv("GRUVI_COMPANY_ID"))
	s.gruviOK, s.gruviTokenURL, s.gruviBFF, s.gruviCompany = s.gruvi.Configurado(), s.gruvi.TokenURL, s.gruvi.BFFURL, s.gruvi.CompanyID
	s.webhookToken = os.Getenv("ATENDE_WEBHOOK_TOKEN")
	s.replyURL = os.Getenv("ATENDE_REPLY_URL")
	s.outbox = &atende.Outbox{}
	if s.replyURL != "" {
		s.outbox.Real = &atende.HTTPSender{URL: s.replyURL, Token: os.Getenv("ATENDE_REPLY_TOKEN")}
	}
	s.provedor = harness.ResolverProvedor(os.Getenv).Nome
	s.atende = &atende.Servico{
		Cat: cat, Agente: env.Ou("ATENDE_AGENTE", "agentes/concierge"), Account: account,
		Config: harness.Config{Nexus: cli, Env: os.Getenv},
		Gruvi:  s.gruvi, Sender: s.outbox, Sessoes: atende.NewSessoes(), Timeout: 120 * time.Second, Log: s.log,
	}

	mux := http.NewServeMux()
	s.rotas(mux)
	s.rotasAtende(mux)

	log.Printf("okf-admin: catálogo %s · account %s · recursos: %s · modelo: %s · gruvi: %v · http://%s", cat.Root, account, s.nexus, s.provedor, s.gruviOK, *addr)
	if *abrir {
		go func() {
			time.Sleep(300 * time.Millisecond)
			abrirNavegador("http://" + *addr)
		}()
	}
	log.Fatal(http.ListenAndServe(*addr, logar(mux)))
}

func (s *servidor) rotas(mux *http.ServeMux) {
	mux.Handle("GET /", semCache(http.FileServerFS(s.web)))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	mux.HandleFunc("GET /api/anatomia", s.anatomia)
	mux.HandleFunc("GET /api/agentes", s.arvore)
	mux.HandleFunc("POST /api/agentes", s.criar)
	mux.HandleFunc("GET /api/agente", s.agente)
	mux.HandleFunc("PUT /api/agente/config", s.config)
	mux.HandleFunc("DELETE /api/agente", s.excluir)
	mux.HandleFunc("POST /api/agentes/mover", s.mover)
	mux.HandleFunc("POST /api/agentes/importar", s.importar)
	mux.HandleFunc("GET /api/agentes/exportar", s.exportar)
	mux.HandleFunc("GET /api/arquivo", s.lerArquivo)
	mux.HandleFunc("PUT /api/arquivo", s.salvarArquivo)
	mux.HandleFunc("DELETE /api/arquivo", s.excluirArquivo)
	mux.HandleFunc("GET /api/arquivo/modelo", s.modelo)
	mux.HandleFunc("GET /api/projecao", s.projecao)
	mux.HandleFunc("GET /api/nexus/recursos", s.recursos)
	mux.HandleFunc("POST /api/agente/recursos", s.vincular)
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
		"pai":           paiRel(a.Rel),
	})
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

func logar(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			log.Printf("%s %s", r.Method, r.URL.RequestURI())
		}
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
