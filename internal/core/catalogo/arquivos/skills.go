package arquivos

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

// ErrFluxoInvalido recusa um fluxo com erros; os problemas apontam os blocos com os IDs do cliente.
type ErrFluxoInvalido struct{ Problemas []ProblemaFluxo }

func (e *ErrFluxoInvalido) Error() string { return "fluxo inválido" }

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
		// Erro de programação: Fluxo e Textos sempre viram JSON (os payloads vêm do YAML com chaves de texto).
		panic(fmt.Sprintf("versão de %T: %v", v, err))
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

// classificador tira um retrato do catálogo: uma consulta à fonte por operação, nunca sob c.mu.
func (c *Catalogo) classificador() classificar {
	return classificadorDe(c.RecursosNexus())
}

func classificadorDe(recursos []Recurso) classificar {
	gravam := map[string]bool{}
	for _, r := range recursos {
		gravam[r.Name] = r.Write
	}
	return func(nome string) (bool, bool) {
		if escrita, ok := gravam[nome]; ok {
			return escrita, true
		}
		return EscritaPeloNome(nome), false
	}
}

func (c *Catalogo) prepararClassificador() func() classificar {
	if c.PrepararRecursos != nil {
		fonte := c.PrepararRecursos()
		return func() classificar { return classificadorDe(fonte()) }
	}
	cl := c.classificador()
	return func() classificar { return cl }
}

// ClassificarFerramenta usa o Write do catálogo; só uma ferramenta fora dele é deduzida pelo nome.
func (c *Catalogo) ClassificarFerramenta(nome string) (escrita, conhecida bool) {
	return c.classificador()(nome)
}

// LerFluxo lê o fluxo do caso.yaml da skill; sem caso.yaml, ErrSemFluxo.
func (c *Catalogo) LerFluxo(rel string) (FluxoLido, error) {
	cl := c.classificador()
	c.mu.Lock()
	defer c.mu.Unlock()
	dir, err := c.resolverSkill(rel)
	if err != nil {
		return FluxoLido{}, fmt.Errorf("ler fluxo %s: %w", rel, err)
	}
	lido, _, err := c.lerFluxo(dir, cl)
	if err != nil {
		return FluxoLido{}, fmt.Errorf("ler fluxo %s: %w", rel, err)
	}
	return lido, nil
}

// lerFluxo devolve também o caso cru, para quem vai regravá-lo.
func (c *Catalogo) lerFluxo(dir string, cl classificar) (FluxoLido, []byte, error) {
	raw, err := c.lerCaso(dir)
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
	classificarAposRecarga := c.prepararClassificador()
	c.mu.Lock()
	defer c.mu.Unlock()
	fim, err := c.iniciarEscrita()
	if err != nil {
		return FluxoGravado{}, err
	}
	defer fim()
	cl := classificarAposRecarga()
	gravado, err := c.salvarFluxo(rel, versao, f, cl)
	if err != nil {
		return FluxoGravado{}, fmt.Errorf("salvar fluxo %s: %w", rel, err)
	}
	if err := c.persistir(); err != nil {
		return FluxoGravado{}, fmt.Errorf("salvar fluxo %s: %w", rel, err)
	}
	return gravado, nil
}

func (c *Catalogo) salvarFluxo(rel, versao string, f Fluxo, cl classificar) (FluxoGravado, error) {
	dir, err := c.resolverSkill(rel)
	if err != nil {
		return FluxoGravado{}, err
	}
	atual, raw, err := c.lerFluxo(dir, cl)
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
	relido, err := c.gravarCaso(dir, novo, cl)
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
	t, err := c.lerTextos(dir)
	if err != nil {
		return Textos{}, "", fmt.Errorf("ler textos %s: %w", rel, err)
	}
	return t, Versao(t), nil
}

func (c *Catalogo) lerTextos(dir string) (Textos, error) {
	raw, err := c.lerCaso(dir)
	if errors.Is(err, ErrSemFluxo) {
		return c.textosDoAgente(dir)
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
		FrasesDeExemplo: caso.Examples, PalavrasChave: caso.Keywords,
		Resumo: caso.Summary, Visibilidade: caso.Visibility, TemFluxo: true,
	}, nil
}

func (c *Catalogo) textosDoAgente(dir string) (Textos, error) {
	cfg, err := c.lerConfig(dir)
	if err != nil {
		return Textos{}, err
	}
	nome, _ := cfg["nome"].(string)
	descricao, _ := cfg["descricao"].(string)
	resumo, _ := cfg["resumo"].(string)
	return Textos{
		Titulo: nome, QuandoUsar: descricao, Resumo: resumo,
		FrasesDeExemplo: stringList(cfg["frases_de_exemplo"]), PalavrasChave: stringList(cfg["palavras_chave"]),
	}, nil
}

// SalvarTextos grava os textos no caso.yaml (quando há); a descrição do agente e a rota no pai
// só mudam quando o "quando usar" ou as palavras-chave mudam. Sem caso.yaml o título não é
// gravado: o nome do agente é o nome da pasta.
func (c *Catalogo) SalvarTextos(rel, versao string, t Textos) (string, error) {
	if strings.TrimSpace(t.Titulo) == "" {
		return "", fmt.Errorf("salvar textos %s: %w", rel, ErrTituloVazio)
	}
	classificarAposRecarga := c.prepararClassificador()
	c.mu.Lock()
	defer c.mu.Unlock()
	fim, err := c.iniciarEscrita()
	if err != nil {
		return "", err
	}
	defer fim()
	cl := classificarAposRecarga()
	nova, err := c.salvarTextos(rel, versao, t, cl)
	if err != nil {
		return "", fmt.Errorf("salvar textos %s: %w", rel, err)
	}
	if err := c.persistir(); err != nil {
		return "", fmt.Errorf("salvar textos %s: %w", rel, err)
	}
	return nova, nil
}

func (c *Catalogo) salvarTextos(rel, versao string, t Textos, cl classificar) (string, error) {
	dir, err := c.resolverSkill(rel)
	if err != nil {
		return "", err
	}
	atual, err := c.lerTextos(dir)
	if err != nil {
		return "", err
	}
	if Versao(atual) != versao {
		return "", ErrVersaoDesatualizada
	}
	if atual.TemFluxo {
		if err := c.gravarTextosNoCaso(dir, atual, t, cl); err != nil {
			return "", err
		}
	} else if err := c.salvarTextosDoAgente(dir, t); err != nil {
		return "", err
	}
	if mudaRoteamento(atual, t) {
		if err := c.atualizarRoteamento(rel, dir, t); err != nil {
			return "", err
		}
	}
	novo, err := c.lerTextos(dir)
	if err != nil {
		return "", err
	}
	return Versao(novo), nil
}

// mudaRoteamento: só o "quando usar" e as palavras-chave alimentam a descrição e a rota.
func mudaRoteamento(atual, novo Textos) bool {
	return atual.QuandoUsar != novo.QuandoUsar || !slices.Equal(atual.PalavrasChave, novo.PalavrasChave)
}

// gravarTextosNoCaso grava os textos no caso.yaml (ver TextosNoCaso).
func (c *Catalogo) gravarTextosNoCaso(dir string, atual, t Textos, cl classificar) error {
	raw, err := c.lerCaso(dir)
	if err != nil {
		return err
	}
	novo, err := TextosNoCaso(raw, atual, t)
	if err != nil {
		return err
	}
	_, err = c.gravarCaso(dir, novo, cl)
	return err
}

// FixarIDs grava o id posicional nos passos do caso.yaml que ainda não têm, e diz se gravou.
// Skill sem fluxo não tem o que fixar.
func (c *Catalogo) FixarIDs(rel string) (bool, error) {
	classificarAposRecarga := c.prepararClassificador()
	c.mu.Lock()
	defer c.mu.Unlock()
	fim, err := c.iniciarEscrita()
	if err != nil {
		return false, err
	}
	defer fim()
	cl := classificarAposRecarga()
	gravou, err := c.fixarIDs(rel, cl)
	if err != nil {
		return false, fmt.Errorf("fixar ids %s: %w", rel, err)
	}
	if !gravou {
		return false, nil
	}
	if err := c.persistir(); err != nil {
		return false, fmt.Errorf("fixar ids %s: %w", rel, err)
	}
	return true, nil
}

func (c *Catalogo) fixarIDs(rel string, cl classificar) (bool, error) {
	dir, err := c.resolverSkill(rel)
	if err != nil {
		return false, err
	}
	raw, err := c.lerCaso(dir)
	if errors.Is(err, ErrSemFluxo) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	novo, mudou, err := FixarIDsNoCaso(raw)
	if err != nil || !mudou {
		return false, err
	}
	if _, err := c.gravarCaso(dir, novo, cl); err != nil {
		return false, err
	}
	return true, nil
}

// salvarTextosDoAgente grava os textos de uma skill sem caso.yaml no agent.yaml; lista
// ou resumo vazio apaga a chave.
func (c *Catalogo) salvarTextosDoAgente(dir string, t Textos) error {
	cfg, err := c.lerConfig(dir)
	if err != nil {
		return err
	}
	definirOuApagar(cfg, "resumo", t.Resumo)
	definirListaOuApagar(cfg, "palavras_chave", t.PalavrasChave)
	definirListaOuApagar(cfg, "frases_de_exemplo", t.FrasesDeExemplo)
	return c.gravarConfig(dir, cfg)
}

func definirOuApagar(cfg map[string]any, chave, valor string) {
	if valor == "" {
		delete(cfg, chave)
		return
	}
	cfg[chave] = valor
}

func definirListaOuApagar(cfg map[string]any, chave string, valores []string) {
	if len(valores) == 0 {
		delete(cfg, chave)
		return
	}
	cfg[chave] = valores
}

// atualizarRoteamento grava a descrição do agente e reescreve, no mesmo lugar, a linha da skill no rotas.md do pai.
func (c *Catalogo) atualizarRoteamento(rel, dir string, t Textos) error {
	if err := c.salvarConfig(rel, map[string]any{"descricao": t.QuandoUsar}); err != nil {
		return err
	}
	pai := c.pai(dir)
	if pai == "" {
		return nil
	}
	if _, err := c.TrocarRota(pai, filepath.Base(dir), criterioDaRota(t)); err != nil {
		return fmt.Errorf("rota: %w", err)
	}
	return nil
}

// criterioDaRota: `<quando usar>; palavras-chave: k1, k2`, para o roteador manter o sinal das palavras.
func criterioDaRota(t Textos) string {
	quando := strings.TrimRight(colapsar(t.QuandoUsar), ".;")
	if len(t.PalavrasChave) == 0 {
		return quando
	}
	palavras := "palavras-chave: " + strings.Join(t.PalavrasChave, ", ")
	if quando == "" {
		return palavras
	}
	return quando + "; " + palavras
}

// CriarSkill cria a pasta da skill dentro do pai, com um fluxo mínimo, e devolve o caminho dela.
func (c *Catalogo) CriarSkill(paiRel, nome, titulo string) (string, error) {
	classificarAposRecarga := c.prepararClassificador()
	c.mu.Lock()
	defer c.mu.Unlock()
	fim, err := c.iniciarEscrita()
	if err != nil {
		return "", err
	}
	defer fim()
	cl := classificarAposRecarga()
	rel, err := c.criarSkill(paiRel, nome, titulo, cl)
	if err != nil {
		return "", fmt.Errorf("criar skill %s: %w", nome, err)
	}
	if err := c.persistir(); err != nil {
		return "", fmt.Errorf("criar skill %s: %w", nome, err)
	}
	return rel, nil
}

func (c *Catalogo) criarSkill(paiRel, nome, titulo string, cl classificar) (string, error) {
	if err := conferirNomeETitulo(nome, titulo); err != nil {
		return "", err
	}
	paiDir, err := c.ResolveAgente(paiRel)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(paiDir, DirAgentes, nome)
	if c.dirExists(dir) {
		return "", ErrSkillJaExiste
	}
	var rel string
	err = c.criarComVolta(dir, func() error {
		var err error
		if rel, _, err = c.criarAgente(NovoAgente{Pai: paiRel, Nome: nome, Descricao: titulo}); err != nil {
			return err
		}
		raw, err := casoNovo(nome, titulo)
		if err != nil {
			return err
		}
		_, err = c.gravarCaso(dir, raw, cl)
		return err
	})
	return rel, err
}

func casoNovo(nome, titulo string) ([]byte, error) {
	no, err := noDe([]par{
		{Chave: "id", Valor: idDoCaso(nome)},
		{Chave: "name", Valor: titulo},
		{Chave: "visibility", Valor: "private"},
		{Chave: "description", Valor: ""},
		{Chave: "keywords", Valor: lista{}},
		{Chave: "examples", Valor: lista{}},
		{Chave: "steps", Valor: seq{[]par{{Chave: "compose", Valor: "Explique ao morador o que esta skill faz."}}}},
		{Chave: "output", Valor: "Pronto! Em que mais posso ajudar?"},
	})
	if err != nil {
		return nil, err
	}
	return codificarCaso(no)
}

// DuplicarSkill copia a skill para uma irmã com nome e título novos e devolve o caminho dela.
func (c *Catalogo) DuplicarSkill(rel, novoNome, novoTitulo string) (string, error) {
	classificarAposRecarga := c.prepararClassificador()
	c.mu.Lock()
	defer c.mu.Unlock()
	fim, err := c.iniciarEscrita()
	if err != nil {
		return "", err
	}
	defer fim()
	cl := classificarAposRecarga()
	novoRel, err := c.duplicarSkill(rel, novoNome, novoTitulo, cl)
	if err != nil {
		return "", fmt.Errorf("duplicar skill %s: %w", rel, err)
	}
	if err := c.persistir(); err != nil {
		return "", fmt.Errorf("duplicar skill %s: %w", rel, err)
	}
	return novoRel, nil
}

func (c *Catalogo) duplicarSkill(rel, novoNome, novoTitulo string, cl classificar) (string, error) {
	if err := conferirNomeETitulo(novoNome, novoTitulo); err != nil {
		return "", err
	}
	origem, err := c.resolverSkill(rel)
	if err != nil {
		return "", err
	}
	destino := filepath.Join(filepath.Dir(origem), novoNome)
	if c.dirExists(destino) {
		return "", ErrSkillJaExiste
	}
	err = c.criarComVolta(destino, func() error {
		if err := c.copiarPasta(origem, destino); err != nil {
			return err
		}
		if err := c.renomearCopia(destino, novoNome, novoTitulo, cl); err != nil {
			return err
		}
		_, err := c.SincronizarRotas("", c.pai(destino), destino, novoNome)
		return err
	})
	if err != nil {
		return "", err
	}
	novoRel, err := filepath.Rel(c.Root, destino)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(novoRel), nil
}

func conferirNomeETitulo(nome, titulo string) error {
	if !SlugOK(nome) {
		return ErrNomeInvalido
	}
	if strings.TrimSpace(titulo) == "" {
		return ErrTituloVazio
	}
	return nil
}

// criarComVolta roda criar; se falhar, apaga a pasta e devolve o rotas.md do pai ao que era,
// para uma nova tentativa não esbarrar em ErrSkillJaExiste nem deixar rota órfã.
func (c *Catalogo) criarComVolta(dir string, criar func() error) error {
	pai := c.pai(dir)
	rotasAntes, errLer := []byte(nil), fs.ErrNotExist
	if pai != "" {
		rotasAntes, errLer = c.readFile(caminhoRotas(pai))
	}
	err := criar()
	if err == nil {
		return nil
	}
	limpeza := c.removeAll(dir)
	if errLer == nil {
		limpeza = errors.Join(limpeza, c.writeFile(caminhoRotas(pai), rotasAntes, 0o644))
	}
	if limpeza != nil {
		return errors.Join(err, fmt.Errorf("desfazer %s: %w", filepath.Base(dir), limpeza))
	}
	return err
}

// renomearCopia troca o nome no agent.yaml e, se houver caso, o id e o name dele.
func (c *Catalogo) renomearCopia(dir, nome, titulo string, cl classificar) error {
	cfg, err := c.lerConfig(dir)
	if err != nil {
		return err
	}
	cfg["nome"] = nome
	if err := c.gravarConfig(dir, cfg); err != nil {
		return err
	}
	raw, err := c.lerCaso(dir)
	if errors.Is(err, ErrSemFluxo) {
		return nil
	}
	if err != nil {
		return err
	}
	chaves, err := chavesDoCaso([]par{{Chave: "id", Valor: idDoCaso(nome)}, {Chave: "name", Valor: titulo}})
	if err != nil {
		return err
	}
	novo, err := substituirChaves(raw, chaves)
	if err != nil {
		return err
	}
	_, err = c.gravarCaso(dir, novo, cl)
	return err
}

// ExcluirSkill apaga a skill e a rota dela no pai; skill que não existe é ErrSkillNaoExiste.
func (c *Catalogo) ExcluirSkill(rel string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	fim, err := c.iniciarEscrita()
	if err != nil {
		return err
	}
	defer fim()
	if _, err := c.resolverSkill(rel); err != nil {
		return fmt.Errorf("excluir skill %s: %w", rel, err)
	}
	remover, err := c.arquivosDoAlvo(rel)
	if err != nil {
		return err
	}
	if _, err := c.excluirAgente(rel); err != nil {
		return fmt.Errorf("excluir skill %s: %w", rel, err)
	}
	if err := c.persistir(remover...); err != nil {
		return fmt.Errorf("excluir skill %s: %w", rel, err)
	}
	return nil
}

func (c *Catalogo) resolverSkill(rel string) (string, error) {
	dir, err := c.ResolveAgente(rel)
	if err != nil {
		return "", err
	}
	if !c.IsAgent(dir) {
		return "", ErrSkillNaoExiste
	}
	return dir, nil
}

func (c *Catalogo) lerCaso(dir string) ([]byte, error) {
	raw, err := c.readFile(filepath.Join(dir, CasoYAML))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrSemFluxo
	}
	return raw, err
}

// gravarCaso confere o caso novo, grava ele e o principal.md derivado dele, e devolve o fluxo relido.
func (c *Catalogo) gravarCaso(dir string, raw []byte, cl classificar) (Fluxo, error) {
	caso, err := ParseCaso(raw)
	if err != nil {
		return Fluxo{}, err
	}
	fluxo, err := FluxoDeYAML(raw, cl.escrita)
	if err != nil {
		return Fluxo{}, err
	}
	principal := filepath.Join(dir, "workflows", "principal.md")
	if err := c.mkdirAll(filepath.Dir(principal), 0o755); err != nil {
		return Fluxo{}, err
	}
	if err := c.writeFile(filepath.Join(dir, CasoYAML), raw, 0o644); err != nil {
		return Fluxo{}, fmt.Errorf("gravar caso: %w", err)
	}
	if err := c.writeFile(principal, []byte(GerarPrincipal(caso.Name, caso.Description, fluxo.Blocos)), 0o644); err != nil {
		return Fluxo{}, fmt.Errorf("gravar principal: %w", err)
	}
	return fluxo, nil
}

func idDoCaso(nome string) string {
	return strings.ReplaceAll(nome, "-", "_")
}
