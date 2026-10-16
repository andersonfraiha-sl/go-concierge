package okf

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Escrita: criar, editar, mover, clonar, importar e exportar pastas de agente.
// Tudo passa por ResolveAgente (nunca sai da raiz) e por caminhoSeguro (nunca
// escreve fora das caixinhas nem dentro de agentes/ de outro).

// extensões aceitas num zip ou num arquivo novo
var extPermitidas = map[string]bool{
	".md": true, ".yaml": true, ".yml": true, ".json": true, ".txt": true, ".csv": true,
	".py": true, ".sh": true, ".go": true, ".sql": true, ".html": true,
	".png": true, ".jpg": true, ".jpeg": true, ".svg": true, ".gif": true, ".webp": true, ".pdf": true,
}

// NovoAgente é o formulário de criação.
type NovoAgente struct {
	Pai       string   `json:"pai"` // caminho relativo do agente pai ("" = raiz)
	Nome      string   `json:"nome"`
	Tipo      string   `json:"tipo"`
	Descricao string   `json:"descricao"`
	Modelo    string   `json:"modelo"`
	Caixinhas []string `json:"caixinhas"` // quais criar com arquivo-modelo
	Fontes    []string `json:"fontes"`
	Apis      []string `json:"apis"`
	Segredos  []string `json:"segredos"`
}

// CriarAgente monta a pasta com os arquivos-modelo e registra a rota no pai.
func (c *Catalogo) CriarAgente(n NovoAgente) (string, []string, error) {
	if !SlugOK(n.Nome) {
		return "", nil, fmt.Errorf("nome inválido: %q (letras minúsculas, dígitos e hífen)", n.Nome)
	}
	tipo := strings.ToLower(strings.TrimSpace(n.Tipo))
	if tipo != "" && tipo != Orquestrador && tipo != Operador {
		return "", nil, errors.New("tipo deve ser orquestrador ou operador")
	}
	base := filepath.Join(c.Root, DirAgentes)
	rel := DirAgentes + "/" + n.Nome
	paiDir := ""
	if n.Pai != "" {
		var err error
		paiDir, err = c.ResolveAgente(n.Pai)
		if err != nil {
			return "", nil, err
		}
		if !IsAgent(paiDir) {
			return "", nil, fmt.Errorf("agente pai não existe: %s", n.Pai)
		}
		base = filepath.Join(paiDir, DirAgentes)
		rel = strings.TrimSuffix(n.Pai, "/") + "/" + DirAgentes + "/" + n.Nome
	}
	dir := filepath.Join(base, n.Nome)
	if dirExists(dir) {
		return "", nil, fmt.Errorf("já existe um agente %q nesse lugar", n.Nome)
	}
	for _, f := range n.Fontes {
		alvo := filepath.Clean(filepath.Join(dir, filepath.FromSlash(f)))
		if !Permitido(dir, alvo) {
			return "", nil, fmt.Errorf("fonte %q aponta para fora do contexto do agente (só a própria pasta ou um ancestral)", f)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", nil, err
	}
	cfg := map[string]any{"nome": n.Nome}
	if tipo != "" {
		cfg["tipo"] = tipo
	}
	cfg["descricao"] = strings.TrimSpace(n.Descricao)
	if n.Modelo != "" {
		cfg["modelo"] = n.Modelo
	}
	if len(n.Fontes) > 0 {
		cfg["fontes"] = n.Fontes
	}
	if len(n.Apis) > 0 {
		cfg["apis"] = n.Apis
	}
	if len(n.Segredos) > 0 {
		cfg["segredos"] = n.Segredos
	}
	if err := gravarConfig(dir, cfg); err != nil {
		return "", nil, err
	}
	mudancas := []string{rel + "/" + AgentYAML}
	for _, cx := range n.Caixinhas {
		for nome, conteudo := range ModelosDaCaixinha(cx, n) {
			p := filepath.Join(dir, filepath.FromSlash(nome))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return "", nil, err
			}
			if err := os.WriteFile(p, []byte(conteudo), 0o644); err != nil {
				return "", nil, err
			}
			mudancas = append(mudancas, rel+"/"+nome)
		}
	}
	if paiDir != "" {
		m, err := SincronizarRotas("", paiDir, dir, n.Nome)
		if err != nil {
			return "", nil, err
		}
		mudancas = append(mudancas, m...)
	}
	return rel, mudancas, nil
}

// caminhoSeguro valida um caminho de arquivo dentro do agente: agent.yaml, ou
// <caixinha>/..., ou okf/... (conhecimento na raiz). Nunca agentes/ (é dos filhos).
func caminhoSeguro(rel string) (string, error) {
	rel = filepath.ToSlash(filepath.Clean("/" + rel))[1:]
	if rel == "" || rel == "." {
		return "", errors.New("caminho vazio")
	}
	if rel == AgentYAML {
		return rel, nil
	}
	primeiro, _, _ := strings.Cut(rel, "/")
	if primeiro == DirAgentes {
		return "", errors.New("agentes/ pertence aos subagentes; edite-os pela árvore")
	}
	if primeiro != "okf" {
		if _, ok := CaixinhaPorNome(primeiro); !ok {
			return "", fmt.Errorf("%q não é uma caixinha do agente", primeiro)
		}
	}
	if !strings.Contains(rel, "/") {
		return "", errors.New("o arquivo precisa ficar dentro de uma caixinha")
	}
	for _, p := range strings.Split(rel, "/") {
		if p == "" || p == "." || p == ".." || strings.HasPrefix(p, ".") {
			return "", errors.New("caminho inválido")
		}
	}
	if !extPermitidas[strings.ToLower(filepath.Ext(rel))] {
		return "", fmt.Errorf("extensão não permitida: %s", filepath.Ext(rel))
	}
	return rel, nil
}

// LerArquivo devolve o conteúdo de um arquivo do agente.
func (c *Catalogo) LerArquivo(agenteRel, rel string) ([]byte, error) {
	dir, err := c.ResolveAgente(agenteRel)
	if err != nil {
		return nil, err
	}
	rel, err = caminhoSeguro(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
}

// SalvarArquivo grava (cria ou substitui). agent.yaml precisa ser YAML válido.
func (c *Catalogo) SalvarArquivo(agenteRel, rel string, conteudo []byte) error {
	dir, err := c.ResolveAgente(agenteRel)
	if err != nil {
		return err
	}
	if !IsAgent(dir) {
		return fmt.Errorf("agente não existe: %s", agenteRel)
	}
	rel, err = caminhoSeguro(rel)
	if err != nil {
		return err
	}
	if rel == AgentYAML {
		var m map[string]any
		if err := yaml.Unmarshal(conteudo, &m); err != nil {
			return fmt.Errorf("agent.yaml inválido: %w", err)
		}
		for _, f := range stringList(m["fontes"]) {
			alvo := filepath.Clean(filepath.Join(dir, filepath.FromSlash(f)))
			if !Permitido(dir, alvo) {
				return fmt.Errorf("fontes: %q aponta para fora do contexto do agente", f)
			}
		}
	}
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, conteudo, 0o644)
}

// ExcluirArquivo remove um arquivo (nunca o agent.yaml).
func (c *Catalogo) ExcluirArquivo(agenteRel, rel string) error {
	dir, err := c.ResolveAgente(agenteRel)
	if err != nil {
		return err
	}
	rel, err = caminhoSeguro(rel)
	if err != nil {
		return err
	}
	if rel == AgentYAML {
		return errors.New("o agent.yaml não se exclui; exclua o agente")
	}
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.Remove(p); err != nil {
		return err
	}
	// caixinha vazia some junto
	_ = os.Remove(filepath.Dir(p))
	return nil
}

// SalvarConfig atualiza os campos principais do agent.yaml preservando o resto.
func (c *Catalogo) SalvarConfig(agenteRel string, campos map[string]any) error {
	dir, err := c.ResolveAgente(agenteRel)
	if err != nil {
		return err
	}
	if !IsAgent(dir) {
		return fmt.Errorf("agente não existe: %s", agenteRel)
	}
	cfg, err := lerConfig(dir)
	if err != nil {
		return err
	}
	for _, k := range []string{"nome", "tipo", "descricao", "modelo"} {
		if v, ok := campos[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if k == "tipo" && s != "" && s != Orquestrador && s != Operador {
				return errors.New("tipo deve ser orquestrador, operador ou vazio (inferido)")
			}
			if s == "" {
				delete(cfg, k)
			} else {
				cfg[k] = s
			}
		}
	}
	for _, k := range []string{"fontes", "apis", "segredos"} {
		if v, ok := campos[k]; ok {
			l := stringList(v)
			if k == "fontes" {
				for _, f := range l {
					alvo := filepath.Clean(filepath.Join(dir, filepath.FromSlash(f)))
					if !Permitido(dir, alvo) {
						return fmt.Errorf("fontes: %q aponta para fora do contexto do agente", f)
					}
				}
			}
			if len(l) == 0 {
				delete(cfg, k)
			} else {
				cfg[k] = l
			}
		}
	}
	return gravarConfig(dir, cfg)
}

// ExcluirAgente apaga a pasta (com os subagentes) e tira a rota do pai.
func (c *Catalogo) ExcluirAgente(agenteRel string) ([]string, error) {
	dir, err := c.ResolveAgente(agenteRel)
	if err != nil {
		return nil, err
	}
	if !IsAgent(dir) {
		return nil, fmt.Errorf("agente não existe: %s", agenteRel)
	}
	pai := Pai(dir)
	nome := filepath.Base(dir)
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	var mudancas []string
	if pai != "" {
		if m, err := RemoverRota(pai, nome); err == nil && m != "" {
			mudancas = append(mudancas, m)
		}
	}
	return mudancas, nil
}

// Mover leva o agente para dentro de outro (destinoRel) ou para a raiz (""),
// e acerta as rotas dos dois pais. Com clonar=true, copia em vez de mover.
func (c *Catalogo) Mover(origemRel, destinoRel string, clonar bool) (string, []string, error) {
	origem, err := c.ResolveAgente(origemRel)
	if err != nil {
		return "", nil, err
	}
	if !IsAgent(origem) {
		return "", nil, fmt.Errorf("agente não existe: %s", origemRel)
	}
	destinoBase := filepath.Join(c.Root, DirAgentes)
	paiNovo := ""
	if destinoRel != "" {
		paiNovo, err = c.ResolveAgente(destinoRel)
		if err != nil {
			return "", nil, err
		}
		if !IsAgent(paiNovo) {
			return "", nil, fmt.Errorf("destino não é um agente: %s", destinoRel)
		}
		destinoBase = filepath.Join(paiNovo, DirAgentes)
	}
	if dentro(origem, destinoBase) {
		return "", nil, errors.New("não dá pra mover um agente pra dentro dele mesmo")
	}
	if filepath.Dir(origem) == destinoBase && !clonar {
		return "", nil, errors.New("o agente já está nesse lugar")
	}
	nome := filepath.Base(origem)
	if clonar {
		if dirExists(filepath.Join(destinoBase, nome)) {
			for n := 2; ; n++ {
				cand := fmt.Sprintf("%s-%d", nome, n)
				if !dirExists(filepath.Join(destinoBase, cand)) {
					nome = cand
					break
				}
			}
		}
	} else if dirExists(filepath.Join(destinoBase, nome)) {
		return "", nil, fmt.Errorf("já existe um agente %q no destino", nome)
	}
	alvo := filepath.Join(destinoBase, nome)
	if err := os.MkdirAll(destinoBase, 0o755); err != nil {
		return "", nil, err
	}
	paiAntigo := Pai(origem)
	if clonar {
		if err := copiarPasta(origem, alvo); err != nil {
			return "", nil, err
		}
		if nome != filepath.Base(origem) {
			cfg, err := lerConfig(alvo)
			if err == nil {
				cfg["nome"] = nome
				_ = gravarConfig(alvo, cfg)
			}
		}
		paiAntigo = ""
	} else if err := os.Rename(origem, alvo); err != nil {
		return "", nil, err
	}
	mudancas, err := SincronizarRotas(paiAntigo, paiNovo, alvo, nome)
	if err != nil {
		return "", nil, err
	}
	rel, _ := filepath.Rel(c.Root, alvo)
	return filepath.ToSlash(rel), mudancas, nil
}

var ignorarNaCopia = map[string]bool{".rag": true, "__pycache__": true, "var": true, ".git": true}

func copiarPasta(origem, destino string) error {
	return filepath.WalkDir(origem, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(origem, p)
		if d.IsDir() {
			if p != origem && ignorarNaCopia[d.Name()] {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(destino, rel), 0o755)
		}
		return copiarArquivo(p, filepath.Join(destino, rel))
	})
}

// ImportarZip instala (ou substitui) um agente a partir do zip da pasta.
// O zip precisa ter agent.yaml ou workflows/principal.md na raiz.
func (c *Catalogo) ImportarZip(agenteRel string, dados []byte) ([]string, error) {
	dir, err := c.ResolveAgente(agenteRel)
	if err != nil {
		return nil, err
	}
	zr, err := zip.NewReader(bytes.NewReader(dados), int64(len(dados)))
	if err != nil {
		return nil, fmt.Errorf("zip inválido: %w", err)
	}
	temp := filepath.Join(filepath.Dir(dir), "."+filepath.Base(dir)+".novo")
	_ = os.RemoveAll(temp)
	var arquivos []string
	for _, f := range zr.File {
		nome := filepath.ToSlash(f.Name)
		if f.FileInfo().IsDir() || strings.HasPrefix(nome, "__MACOSX") {
			continue
		}
		partes := strings.Split(nome, "/")
		limpo := []string{}
		for _, p := range partes {
			if p == "" || p == "." || p == ".." {
				continue
			}
			limpo = append(limpo, p)
		}
		if len(limpo) == 0 {
			continue
		}
		if ocultoOuLixo(limpo) {
			continue
		}
		if !extPermitidas[strings.ToLower(filepath.Ext(limpo[len(limpo)-1]))] {
			continue
		}
		alvo := filepath.Join(temp, filepath.Join(limpo...))
		if !dentro(temp, alvo) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(alvo), 0o755); err != nil {
			return nil, err
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		out, err := os.Create(alvo)
		if err != nil {
			rc.Close()
			return nil, err
		}
		_, err = io.Copy(out, io.LimitReader(rc, 50<<20))
		rc.Close()
		out.Close()
		if err != nil {
			return nil, err
		}
		arquivos = append(arquivos, strings.Join(limpo, "/"))
	}
	// zip com uma pasta única na raiz: desce um nível
	if !IsAgent(temp) {
		entradas, _ := os.ReadDir(temp)
		if len(entradas) == 1 && entradas[0].IsDir() && IsAgent(filepath.Join(temp, entradas[0].Name())) {
			interno := filepath.Join(temp, entradas[0].Name())
			temp2 := temp + ".2"
			if err := os.Rename(interno, temp2); err == nil {
				_ = os.RemoveAll(temp)
				temp = temp2
			}
		}
	}
	if !IsAgent(temp) {
		_ = os.RemoveAll(temp)
		return nil, errors.New("o zip precisa ter agent.yaml ou workflows/principal.md na raiz")
	}
	_ = os.RemoveAll(dir)
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, err
	}
	if err := os.Rename(temp, dir); err != nil {
		return nil, err
	}
	if pai := Pai(dir); pai != "" {
		_, _ = SincronizarRotas("", pai, dir, filepath.Base(dir))
	}
	return arquivos, nil
}

func ocultoOuLixo(partes []string) bool {
	for _, p := range partes {
		if strings.HasPrefix(p, ".") || p == "__pycache__" || p == "node_modules" || p == "var" {
			return true
		}
	}
	return false
}

// ExportarZip empacota a pasta do agente (sem caches).
func (c *Catalogo) ExportarZip(agenteRel string) ([]byte, error) {
	dir, err := c.ResolveAgente(agenteRel)
	if err != nil {
		return nil, err
	}
	if !IsAgent(dir) {
		return nil, fmt.Errorf("agente não existe: %s", agenteRel)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != dir && (ignorarNaCopia[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
