// okf-admin: a interface de cadastro de skills (agentes e subagentes) no formato
// de pasta, com os vínculos internos de assets, dados, rotas e subagentes.
//
//	okf-admin -root ./okf -addr 127.0.0.1:8090
//
// É a mesma ideia do playground do adam (Python), reescrita em Go e sem executor:
// aqui só se cadastra, valida e projeta. Quem executa é o harness Eino.
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

	"go-concierge/internal/okf"
	"go-concierge/web"
)

type servidor struct {
	cat *okf.Catalogo
	web fs.FS
}

func main() {
	root := flag.String("root", "okf", "raiz do catálogo (agentes/, bases/, tools/, apis/)")
	addr := flag.String("addr", "127.0.0.1:8090", "endereço HTTP")
	abrir := flag.Bool("open", false, "abre o navegador")
	webDir := flag.String("web", "", "serve a interface desta pasta em vez da embutida (desenvolvimento: web/okf)")
	flag.Parse()

	cat, err := okf.NewCatalogo(*root)
	if err != nil {
		log.Fatal(err)
	}
	s := &servidor{cat: cat, web: web.OKF}
	if *webDir != "" {
		s.web = os.DirFS(*webDir)
	}
	mux := http.NewServeMux()
	s.rotas(mux)

	log.Printf("okf-admin: catálogo %s em http://%s", cat.Root, *addr)
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
	mux.HandleFunc("GET /api/catalogo", s.catalogo)
	mux.HandleFunc("POST /api/recursos/aplicar", s.aplicar)
}

// ---- handlers ------------------------------------------------------------

func (s *servidor) anatomia(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{"caixinhas": okf.Caixinhas, "tipos_okf": okf.TiposOKF})
}

func (s *servidor) arvore(w http.ResponseWriter, _ *http.Request) {
	tree, err := s.cat.Tree()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"agentes": resumir(tree)})
}

// nó resumido da árvore (sem arquivos), com contagem de problemas
type no struct {
	Caminho    string `json:"caminho"`
	Nome       string `json:"nome"`
	Tipo       string `json:"tipo"`
	Descricao  string `json:"descricao"`
	Erros      int    `json:"erros"`
	Avisos     int    `json:"avisos"`
	Subagentes []no   `json:"subagentes"`
}

func resumir(tree []*okf.Agent) []no {
	out := []no{}
	for _, a := range tree {
		n := no{Caminho: a.Rel, Nome: a.Nome, Tipo: a.Tipo, Descricao: a.Descricao, Subagentes: resumir(a.Subagentes)}
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
		"conhecimento":  s.cat.Conhecimento(a),
		"dados":         s.cat.Dados(a),
		"procedimentos": s.cat.Procedimentos(a),
		"segredos":      s.cat.SegredosDeclarados(a),
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
	chave := "movido"
	if corpo.Clonar {
		chave = "clonado"
	}
	writeJSON(w, 200, map[string]any{chave: rel, "caminho": rel, "mudancas": mud})
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
	if q.Get("bruto") == "1" {
		http.ServeContent(w, r, filepath.Base(q.Get("caminho")), time.Time{}, strings.NewReader(string(raw)))
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

func (s *servidor) catalogo(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, 200, map[string]any{
		"bases": orVazio(s.cat.ListarBases()),
		"tools": s.cat.ListarTools(),
		"apis":  orVazio(s.cat.ListarAPIs()),
	})
}

func orVazio[T any](l []T) []T {
	if l == nil {
		return []T{}
	}
	return l
}

func (s *servidor) aplicar(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Tipo   string `json:"tipo"`
		Nome   string `json:"nome"`
		Agente string `json:"agente"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		writeErr(w, 400, err)
		return
	}
	mud, err := s.cat.Aplicar(corpo.Agente, corpo.Tipo, corpo.Nome)
	if err != nil {
		code := 400
		if strings.Contains(err.Error(), "desconhec") {
			code = 404
		}
		writeErr(w, code, err)
		return
	}
	writeJSON(w, 200, map[string]any{"aplicado": corpo.Tipo + ":" + corpo.Nome, "mudancas": mud})
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
