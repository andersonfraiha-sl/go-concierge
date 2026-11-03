package okf

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Skills vistas pelo Studio: o fluxo e os textos de cada uma, lidos e gravados com
// conferência de versão (quem grava em cima de uma versão velha é recusado), mais criar
// e duplicar. Toda gravação do caso.yaml regera o workflows/principal.md junto.

var (
	ErrSemFluxo            = errors.New("skill sem fluxo")
	ErrVersaoDesatualizada = errors.New("versão desatualizada")
	ErrNomeInvalido        = errors.New("nome inválido")
	ErrSkillJaExiste       = errors.New("skill já existe")
)

// ErrFluxoInvalido recusa um fluxo com erros; os problemas apontam os blocos com os IDs do cliente.
type ErrFluxoInvalido struct{ Problemas []ProblemaFluxo }

func (e *ErrFluxoInvalido) Error() string { return "fluxo inválido" }

// Textos são as partes da skill que o Studio edita fora do fluxo.
type Textos struct {
	Titulo          string   `json:"titulo"`          // caso.name (sem caso: agent.yaml nome)
	QuandoUsar      string   `json:"quandoUsar"`      // caso.description (sem caso: agent.yaml descricao)
	FrasesDeExemplo []string `json:"frasesDeExemplo"` // caso.examples
	PalavrasChave   []string `json:"palavrasChave"`   // caso.keywords
	TemFluxo        bool     `json:"temFluxo"`        // só leitura
}

// FluxoLido é o fluxo de uma skill com a versão dele e os problemas que já tem.
type FluxoLido struct {
	Fluxo     Fluxo           `json:"fluxo"`
	Versao    string          `json:"versao"`
	Problemas []ProblemaFluxo `json:"problemas"` // ValidarFluxo + FerramentasDesconhecidas
}

// FluxoGravado é a versão nova depois de gravar, com os avisos sobre o fluxo enviado.
type FluxoGravado struct {
	Versao    string          `json:"versao"`
	Problemas []ProblemaFluxo `json:"problemas"` // problemas com os IDs do cliente
}

// Versao identifica o conteúdo: 16 dígitos hex do sha256 do JSON.
func Versao(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		// payload que o JSON não representa (mapa com chave não textual): fmt ordena os mapas, então segue determinístico
		raw = fmt.Appendf(nil, "%#v", v)
	}
	soma := sha256.Sum256(raw)
	return hex.EncodeToString(soma[:])[:16]
}

// classificar diz se uma ferramenta grava no sistema e se o catálogo a conhece.
type classificar func(nome string) (escrita, conhecida bool)

func (cl classificar) escrita(nome string) bool {
	escrita, _ := cl(nome)
	return escrita
}

func (cl classificar) conhecida(nome string) bool {
	_, conhecida := cl(nome)
	return conhecida
}

// classificador tira um retrato do catálogo: uma consulta à fonte por operação, não por ferramenta.
func (c *Catalogo) classificador() classificar {
	gravam := map[string]bool{}
	for _, r := range c.RecursosNexus() {
		gravam[r.Name] = r.Write
	}
	return func(nome string) (bool, bool) {
		if escrita, ok := gravam[nome]; ok {
			return escrita, true
		}
		return EscritaPeloNome(nome), false
	}
}

// ClassificarFerramenta usa o Write do catálogo; só uma ferramenta fora dele é deduzida pelo nome.
func (c *Catalogo) ClassificarFerramenta(nome string) (escrita, conhecida bool) {
	return c.classificador()(nome)
}

// LerFluxo lê o fluxo do caso.yaml da skill; sem caso.yaml, ErrSemFluxo.
func (c *Catalogo) LerFluxo(rel string) (FluxoLido, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	dir, err := c.resolverSkill(rel)
	if err != nil {
		return FluxoLido{}, fmt.Errorf("ler fluxo %s: %w", rel, err)
	}
	lido, _, err := lerFluxo(dir, c.classificador())
	if err != nil {
		return FluxoLido{}, fmt.Errorf("ler fluxo %s: %w", rel, err)
	}
	return lido, nil
}

// lerFluxo devolve também o caso cru, para quem vai regravá-lo.
func lerFluxo(dir string, cl classificar) (FluxoLido, []byte, error) {
	raw, err := lerCaso(dir)
	if err != nil {
		return FluxoLido{}, nil, err
	}
	f, err := FluxoDeYAML(raw, cl.escrita)
	if err != nil {
		return FluxoLido{}, nil, err
	}
	problemas := append(ValidarFluxo(f), FerramentasDesconhecidas(f, cl.conhecida)...)
	return FluxoLido{Fluxo: f, Versao: Versao(f), Problemas: problemas}, raw, nil
}

// SalvarFluxo grava o fluxo se a versão for a atual e o fluxo não tiver erro; o tipo
// buscar/gravar de cada ferramenta é decidido aqui, não pelo cliente.
func (c *Catalogo) SalvarFluxo(rel, versao string, f Fluxo) (FluxoGravado, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	gravado, err := c.salvarFluxo(rel, versao, f)
	if err != nil {
		return FluxoGravado{}, fmt.Errorf("salvar fluxo %s: %w", rel, err)
	}
	return gravado, nil
}

func (c *Catalogo) salvarFluxo(rel, versao string, f Fluxo) (FluxoGravado, error) {
	dir, err := c.resolverSkill(rel)
	if err != nil {
		return FluxoGravado{}, err
	}
	cl := c.classificador()
	atual, raw, err := lerFluxo(dir, cl)
	if err != nil {
		return FluxoGravado{}, err
	}
	if atual.Versao != versao {
		return FluxoGravado{}, ErrVersaoDesatualizada
	}
	f = Reclassificar(f, cl.escrita)
	if problemas := ValidarFluxo(f); len(problemas) > 0 {
		return FluxoGravado{}, &ErrFluxoInvalido{Problemas: problemas}
	}
	novo, err := AplicarFluxo(raw, f)
	if err != nil {
		return FluxoGravado{}, err
	}
	relido, err := gravarCaso(dir, novo, cl)
	if err != nil {
		return FluxoGravado{}, err
	}
	avisos := append([]ProblemaFluxo{}, FerramentasDesconhecidas(f, cl.conhecida)...)
	return FluxoGravado{Versao: Versao(relido), Problemas: avisos}, nil
}

// LerTextos devolve os textos da skill e a versão deles.
func (c *Catalogo) LerTextos(rel string) (Textos, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	dir, err := c.resolverSkill(rel)
	if err != nil {
		return Textos{}, "", fmt.Errorf("ler textos %s: %w", rel, err)
	}
	t, err := lerTextos(dir)
	if err != nil {
		return Textos{}, "", fmt.Errorf("ler textos %s: %w", rel, err)
	}
	return t, Versao(t), nil
}

func lerTextos(dir string) (Textos, error) {
	raw, err := lerCaso(dir)
	if errors.Is(err, ErrSemFluxo) {
		return textosDoAgente(dir)
	}
	if err != nil {
		return Textos{}, err
	}
	caso, err := ParseCaso(raw)
	if err != nil {
		return Textos{}, err
	}
	return Textos{
		Titulo: caso.Name, QuandoUsar: caso.Description,
		FrasesDeExemplo: caso.Examples, PalavrasChave: caso.Keywords, TemFluxo: true,
	}, nil
}

func textosDoAgente(dir string) (Textos, error) {
	cfg, err := lerConfig(dir)
	if err != nil {
		return Textos{}, err
	}
	nome, _ := cfg["nome"].(string)
	descricao, _ := cfg["descricao"].(string)
	return Textos{Titulo: nome, QuandoUsar: descricao, FrasesDeExemplo: []string{}, PalavrasChave: []string{}}, nil
}

// SalvarTextos grava os textos no caso.yaml (quando há), na descrição do agente e na rota do pai.
// Sem caso.yaml o título não é gravado: o nome do agente é o nome da pasta.
func (c *Catalogo) SalvarTextos(rel, versao string, t Textos) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	nova, err := c.salvarTextos(rel, versao, t)
	if err != nil {
		return "", fmt.Errorf("salvar textos %s: %w", rel, err)
	}
	return nova, nil
}

func (c *Catalogo) salvarTextos(rel, versao string, t Textos) (string, error) {
	dir, err := c.resolverSkill(rel)
	if err != nil {
		return "", err
	}
	atual, err := lerTextos(dir)
	if err != nil {
		return "", err
	}
	if Versao(atual) != versao {
		return "", ErrVersaoDesatualizada
	}
	if atual.TemFluxo {
		if err := gravarTextosNoCaso(dir, t, c.classificador()); err != nil {
			return "", err
		}
	}
	if err := c.SalvarConfig(rel, map[string]any{"descricao": t.QuandoUsar}); err != nil {
		return "", err
	}
	if err := atualizarRota(dir, t.QuandoUsar); err != nil {
		return "", err
	}
	novo, err := lerTextos(dir)
	if err != nil {
		return "", err
	}
	return Versao(novo), nil
}

func gravarTextosNoCaso(dir string, t Textos, cl classificar) error {
	raw, err := lerCaso(dir)
	if err != nil {
		return err
	}
	frases := make(seq, len(t.FrasesDeExemplo))
	for i, frase := range t.FrasesDeExemplo {
		frases[i] = frase
	}
	chaves, err := chavesDoCaso([]par{
		{"name", t.Titulo}, {"description", t.QuandoUsar}, {"examples", frases}, {"keywords", lista(t.PalavrasChave)},
	})
	if err != nil {
		return err
	}
	novo, err := substituirChaves(raw, chaves)
	if err != nil {
		return err
	}
	_, err = gravarCaso(dir, novo, cl)
	return err
}

// atualizarRota reescreve a linha da skill no rotas.md do pai com o "quando usar" novo.
func atualizarRota(dir, quandoUsar string) error {
	pai := Pai(dir)
	if pai == "" {
		return nil
	}
	nome := filepath.Base(dir)
	if _, err := RemoverRota(pai, nome); err != nil {
		return fmt.Errorf("rota: %w", err)
	}
	if _, err := RegistrarRota(pai, nome, colapsar(quandoUsar)); err != nil {
		return fmt.Errorf("rota: %w", err)
	}
	return nil
}

// CriarSkill cria a pasta da skill dentro do pai, com um fluxo mínimo, e devolve o caminho dela.
func (c *Catalogo) CriarSkill(paiRel, nome, titulo string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rel, err := c.criarSkill(paiRel, nome, titulo)
	if err != nil {
		return "", fmt.Errorf("criar skill %s: %w", nome, err)
	}
	return rel, nil
}

func (c *Catalogo) criarSkill(paiRel, nome, titulo string) (string, error) {
	if !SlugOK(nome) {
		return "", ErrNomeInvalido
	}
	paiDir, err := c.ResolveAgente(paiRel)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(paiDir, DirAgentes, nome)
	if dirExists(dir) {
		return "", ErrSkillJaExiste
	}
	rel, _, err := c.CriarAgente(NovoAgente{Pai: paiRel, Nome: nome, Descricao: titulo})
	if err != nil {
		return "", err
	}
	raw, err := casoNovo(nome, titulo)
	if err != nil {
		return "", err
	}
	if _, err := gravarCaso(dir, raw, c.classificador()); err != nil {
		return "", err
	}
	return rel, nil
}

func casoNovo(nome, titulo string) ([]byte, error) {
	no, err := noDe([]par{
		{"id", idDoCaso(nome)},
		{"name", titulo},
		{"visibility", "private"},
		{"description", ""},
		{"keywords", lista{}},
		{"examples", lista{}},
		{"steps", seq{[]par{{"compose", "Explique ao morador o que esta skill faz."}}}},
		{"output", "Pronto! Em que mais posso ajudar?"},
	})
	if err != nil {
		return nil, err
	}
	return codificarCaso(no)
}

// DuplicarSkill copia a skill para uma irmã com nome e título novos e devolve o caminho dela.
func (c *Catalogo) DuplicarSkill(rel, novoNome, novoTitulo string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	novoRel, err := c.duplicarSkill(rel, novoNome, novoTitulo)
	if err != nil {
		return "", fmt.Errorf("duplicar skill %s: %w", rel, err)
	}
	return novoRel, nil
}

func (c *Catalogo) duplicarSkill(rel, novoNome, novoTitulo string) (string, error) {
	if !SlugOK(novoNome) {
		return "", ErrNomeInvalido
	}
	origem, err := c.resolverSkill(rel)
	if err != nil {
		return "", err
	}
	destino := filepath.Join(filepath.Dir(origem), novoNome)
	if dirExists(destino) {
		return "", ErrSkillJaExiste
	}
	if err := copiarPasta(origem, destino); err != nil {
		return "", err
	}
	if err := renomearCopia(destino, novoNome, novoTitulo, c.classificador()); err != nil {
		return "", err
	}
	if _, err := SincronizarRotas("", Pai(destino), destino, novoNome); err != nil {
		return "", err
	}
	novoRel, err := filepath.Rel(c.Root, destino)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(novoRel), nil
}

// renomearCopia troca o nome no agent.yaml e, se houver caso, o id e o name dele.
func renomearCopia(dir, nome, titulo string, cl classificar) error {
	cfg, err := lerConfig(dir)
	if err != nil {
		return err
	}
	cfg["nome"] = nome
	if err := gravarConfig(dir, cfg); err != nil {
		return err
	}
	raw, err := lerCaso(dir)
	if errors.Is(err, ErrSemFluxo) {
		return nil
	}
	if err != nil {
		return err
	}
	chaves, err := chavesDoCaso([]par{{"id", idDoCaso(nome)}, {"name", titulo}})
	if err != nil {
		return err
	}
	novo, err := substituirChaves(raw, chaves)
	if err != nil {
		return err
	}
	_, err = gravarCaso(dir, novo, cl)
	return err
}

// --- bordas de arquivo ---

func (c *Catalogo) resolverSkill(rel string) (string, error) {
	dir, err := c.ResolveAgente(rel)
	if err != nil {
		return "", err
	}
	if !IsAgent(dir) {
		return "", fmt.Errorf("skill não existe: %w", fs.ErrNotExist)
	}
	return dir, nil
}

func lerCaso(dir string) ([]byte, error) {
	raw, err := os.ReadFile(filepath.Join(dir, CasoYAML))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrSemFluxo
	}
	return raw, err
}

// gravarCaso confere o caso novo, grava ele e o principal.md derivado dele, e devolve o fluxo relido.
func gravarCaso(dir string, raw []byte, cl classificar) (Fluxo, error) {
	caso, err := ParseCaso(raw)
	if err != nil {
		return Fluxo{}, err
	}
	fluxo, err := FluxoDeYAML(raw, cl.escrita)
	if err != nil {
		return Fluxo{}, err
	}
	principal := filepath.Join(dir, "workflows", "principal.md")
	if err := os.MkdirAll(filepath.Dir(principal), 0o755); err != nil {
		return Fluxo{}, err
	}
	if err := os.WriteFile(filepath.Join(dir, CasoYAML), raw, 0o644); err != nil {
		return Fluxo{}, fmt.Errorf("gravar caso: %w", err)
	}
	if err := os.WriteFile(principal, []byte(GerarPrincipal(caso.Name, caso.Description, fluxo.Blocos)), 0o644); err != nil {
		return Fluxo{}, fmt.Errorf("gravar principal: %w", err)
	}
	return fluxo, nil
}

func chavesDoCaso(pares []par) ([]chaveNo, error) {
	chaves := make([]chaveNo, len(pares))
	for i, p := range pares {
		no, err := noDe(p.valor)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p.chave, err)
		}
		chaves[i] = chaveNo{p.chave, no}
	}
	return chaves, nil
}

func idDoCaso(nome string) string {
	return strings.ReplaceAll(nome, "-", "_")
}
