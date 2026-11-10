package okf

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"go-concierge/internal/catalogo"
)

// ErrLoteInvalido é o lote de arquivos que nem chega a ser tentado: caminho fora de agentes/,
// repetido, ou lote vazio. Quem responde ao cliente trata como pedido ruim.
var ErrLoteInvalido = errors.New("lote de arquivos inválido")

// ArquivoBruto é um arquivo a gravar: caminho relativo à raiz, com barras, e os bytes como são.
type ArquivoBruto struct {
	Caminho  string
	Conteudo []byte
}

// ResultadoDoLote é o que AplicarArquivos fez. Com Problemas, nada foi aplicado.
type ResultadoDoLote struct {
	Gravados  int
	Removidos int // só os que existiam: remover o que não existe não conta
	Problemas []Problema
}

// ValidarCaminhoDeArquivo aceita só o que o lote pode tocar: arquivo limpo sob agentes/, sem
// segmento oculto. nexus/ fica de fora: o instantâneo de recursos tem dono próprio.
func ValidarCaminhoDeArquivo(caminho string) error {
	if err := catalogo.ValidarCaminho(caminho); err != nil {
		return err
	}
	if !strings.HasPrefix(caminho, DirAgentes+"/") {
		return fmt.Errorf("%w: %q (esta rota só aceita agentes/...)", catalogo.ErrCaminhoInvalido, caminho)
	}
	if slices.ContainsFunc(strings.Split(caminho, "/"), func(s string) bool { return strings.HasPrefix(s, ".") }) {
		return fmt.Errorf("%w: %q tem segmento oculto", catalogo.ErrCaminhoInvalido, caminho)
	}
	return nil
}

// ValidarLote confere o lote sem tocar em disco: não vazio, caminhos válidos, nenhum repetido
// (nem entre gravar e remover) e nenhum caminho que seja a pasta de outro do lote.
func ValidarLote(gravar []ArquivoBruto, remover []string) error {
	caminhos := slices.Concat(caminhosDe(gravar), remover)
	if len(caminhos) == 0 {
		return fmt.Errorf("%w: nada a gravar nem a remover", ErrLoteInvalido)
	}
	vistos := make(map[string]bool, len(caminhos))
	for _, c := range caminhos {
		if err := ValidarCaminhoDeArquivo(c); err != nil {
			return fmt.Errorf("%w: %w", ErrLoteInvalido, err)
		}
		if vistos[c] {
			return fmt.Errorf("%w: %q repetido", ErrLoteInvalido, c)
		}
		vistos[c] = true
	}
	for _, c := range caminhos {
		for pai := path.Dir(c); pai != "."; pai = path.Dir(pai) {
			if vistos[pai] {
				return fmt.Errorf("%w: %q é arquivo e pasta ao mesmo tempo (%q)", ErrLoteInvalido, pai, c)
			}
		}
	}
	return nil
}

func caminhosDe(arquivos []ArquivoBruto) []string {
	out := make([]string, len(arquivos))
	for i, a := range arquivos {
		out[i] = a.Caminho
	}
	return out
}

// AplicarArquivos grava e remove o lote como uma unidade, sob a trava do catálogo: guarda o
// estado anterior, escreve, recarrega a árvore e, se ela não carrega ou ganhou problema de
// nível erro, desfaz e devolve os problemas novos. Se carrega, persiste; falha do gancho
// também desfaz. Depois de desfazer, disco e banco estão como antes da chamada. O erro
// devolvido é falha do servidor (disco, banco, desfazer); pedido inaceitável é ErrLoteInvalido.
func (c *Catalogo) AplicarArquivos(gravar []ArquivoBruto, remover []string) (ResultadoDoLote, error) {
	if err := ValidarLote(gravar, remover); err != nil {
		return ResultadoDoLote{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	foto, conflitos, err := c.fotografar(caminhosDe(gravar), remover)
	if err != nil || len(conflitos) > 0 {
		return ResultadoDoLote{Problemas: conflitos}, err
	}
	antes := c.errosDaArvore()

	removidos, err := c.escreverLote(gravar, foto, remover)
	if err != nil {
		return ResultadoDoLote{}, c.desfazerComErro(foto, fmt.Errorf("aplicar arquivos: %w", err), false)
	}
	if novos := problemasNovos(c.errosDaArvore(), antes); len(novos) > 0 {
		if err := c.desfazer(foto); err != nil {
			return ResultadoDoLote{}, fmt.Errorf("desfazer lote que quebrou o catálogo: %w", err)
		}
		return ResultadoDoLote{Problemas: novos}, nil
	}
	if err := c.persistir(); err != nil {
		return ResultadoDoLote{}, c.desfazerComErro(foto, err, true)
	}
	return ResultadoDoLote{Gravados: len(gravar), Removidos: removidos}, nil
}

// desfazerComErro desfaz no disco e devolve causa; se o desfazer falha, o erro dele vai junto.
// Com repersistir, devolve o banco ao estado do disco restaurado: a falha de persistência pode
// ter sido depois do commit (timeout), e o gancho compara o catálogo inteiro.
func (c *Catalogo) desfazerComErro(foto fotografia, causa error, repersistir bool) error {
	erro := causa
	if err := c.desfazer(foto); err != nil {
		return errors.Join(erro, fmt.Errorf("desfazer: %w", err))
	}
	if repersistir {
		if err := c.persistir(); err != nil {
			erro = errors.Join(erro, fmt.Errorf("repersistir depois de desfazer: %w", err))
		}
	}
	return erro
}

// ---- estado anterior ---------------------------------------------------------

type estadoDoArquivo struct {
	existia  bool
	conteudo []byte
}

// fotografia é o que o desfazer precisa: o conteúdo anterior de cada caminho tocado e as
// pastas que o lote vai criar (para sumirem se o lote for desfeito).
type fotografia struct {
	arquivos    map[string]estadoDoArquivo // por caminho relativo à raiz
	pastasNovas []string                   // absolutas, da mais funda para a mais rasa
}

// fotografar lê o estado anterior de cada caminho. Caminho que já é pasta ou outro tipo de
// arquivo, ou cujo pai é arquivo, volta como problema: o lote não pode ser aplicado.
func (c *Catalogo) fotografar(gravar, remover []string) (fotografia, []Problema, error) {
	foto := fotografia{arquivos: map[string]estadoDoArquivo{}}
	var conflitos []Problema
	for _, rel := range slices.Concat(gravar, remover) {
		abs, err := c.caminhoNoDisco(rel)
		if err != nil {
			return fotografia{}, nil, err
		}
		if pai, ok := paiQueNaoEPasta(abs, c.Root); ok {
			conflitos = append(conflitos, conflito(rel, fmt.Sprintf("%s existe e não é uma pasta", pai)))
			continue
		}
		estado, texto, err := lerEstado(abs)
		if err != nil {
			return fotografia{}, nil, fmt.Errorf("ler estado anterior de %s: %w", rel, err)
		}
		if texto != "" {
			conflitos = append(conflitos, conflito(rel, texto))
			continue
		}
		foto.arquivos[rel] = estado
	}
	if len(conflitos) > 0 {
		return fotografia{}, conflitos, nil
	}
	foto.pastasNovas = c.pastasQueFaltam(gravar)
	return foto, nil, nil
}

func conflito(rel, texto string) Problema {
	return Problema{Nivel: "erro", Onde: rel, Texto: texto, Solucao: "use outro caminho"}
}

// lerEstado devolve o estado de abs; texto não vazio é o motivo de não ser um arquivo comum.
func lerEstado(abs string) (estado estadoDoArquivo, texto string, err error) {
	info, err := os.Lstat(abs)
	switch {
	case ausente(err):
		return estadoDoArquivo{}, "", nil
	case err != nil:
		return estadoDoArquivo{}, "", err
	case !info.Mode().IsRegular():
		return estadoDoArquivo{}, "já existe e não é um arquivo comum", nil
	}
	conteudo, err := os.ReadFile(abs)
	return estadoDoArquivo{existia: true, conteudo: conteudo}, "", err
}

func ausente(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// paiQueNaoEPasta acha, subindo de abs até a raiz do catálogo, um ancestral que existe e não
// é pasta (gravar sob um arquivo).
func paiQueNaoEPasta(abs, raiz string) (string, bool) {
	for pai := filepath.Dir(abs); pai != raiz && dentro(raiz, pai); pai = filepath.Dir(pai) {
		info, err := os.Lstat(pai)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(raiz, pai)
			return filepath.ToSlash(rel), true
		}
		return "", false
	}
	return "", false
}

// pastasQueFaltam lista as pastas sob agentes/ que os caminhos precisam e ainda não existem.
func (c *Catalogo) pastasQueFaltam(caminhos []string) []string {
	base := filepath.Join(c.Root, DirAgentes)
	faltam := map[string]bool{}
	for _, rel := range caminhos {
		for pasta := filepath.Dir(filepath.Join(c.Root, filepath.FromSlash(rel))); pasta != base && dentro(base, pasta); pasta = filepath.Dir(pasta) {
			if dirExists(pasta) {
				break
			}
			faltam[pasta] = true
		}
	}
	out := make([]string, 0, len(faltam))
	for pasta := range faltam {
		out = append(out, pasta)
	}
	slices.SortFunc(out, func(a, b string) int { return len(b) - len(a) })
	return out
}

// caminhoNoDisco resolve rel dentro de <raiz>/agentes. É a segunda barreira, depois de
// ValidarCaminhoDeArquivo: nada é escrito nem apagado fora dessa pasta.
func (c *Catalogo) caminhoNoDisco(rel string) (string, error) {
	if err := ValidarCaminhoDeArquivo(rel); err != nil {
		return "", fmt.Errorf("%w: %w", ErrLoteInvalido, err)
	}
	abs := filepath.Join(c.Root, filepath.FromSlash(rel))
	if !dentro(filepath.Join(c.Root, DirAgentes), abs) {
		return "", fmt.Errorf("%w: %q sai de agentes/", ErrLoteInvalido, rel)
	}
	return abs, nil
}

// ---- escrita e desfazer --------------------------------------------------------

// escreverLote grava e remove no disco e devolve quantos arquivos existentes foram removidos.
func (c *Catalogo) escreverLote(gravar []ArquivoBruto, foto fotografia, remover []string) (int, error) {
	for _, a := range gravar {
		abs, err := c.caminhoNoDisco(a.Caminho)
		if err != nil {
			return 0, err
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return 0, fmt.Errorf("criar pasta de %s: %w", a.Caminho, err)
		}
		if err := escreverArquivoAtomico(abs, a.Conteudo, 0o644); err != nil {
			return 0, fmt.Errorf("gravar %s: %w", a.Caminho, err)
		}
	}
	removidos := 0
	for _, rel := range remover {
		if !foto.arquivos[rel].existia {
			continue
		}
		abs, err := c.caminhoNoDisco(rel)
		if err != nil {
			return removidos, err
		}
		if err := os.Remove(abs); err != nil {
			return removidos, fmt.Errorf("remover %s: %w", rel, err)
		}
		removidos++
		c.podarPastasVazias(abs)
	}
	return removidos, nil
}

// podarPastasVazias tira as pastas que ficaram vazias depois de remover um arquivo, de baixo
// para cima, sem nunca tocar em agentes/. os.Remove recusa pasta com conteúdo, o que encerra a poda.
func (c *Catalogo) podarPastasVazias(abs string) {
	base := filepath.Join(c.Root, DirAgentes)
	for pasta := filepath.Dir(abs); pasta != base && dentro(base, pasta); pasta = filepath.Dir(pasta) {
		if os.Remove(pasta) != nil {
			return
		}
	}
}

// desfazer devolve cada caminho tocado ao conteúdo anterior (ou à inexistência) e tira as
// pastas que o lote criou. Tenta tudo e devolve todos os erros juntos.
func (c *Catalogo) desfazer(foto fotografia) error {
	var erros []error
	for rel, estado := range foto.arquivos {
		abs, err := c.caminhoNoDisco(rel)
		if err != nil {
			erros = append(erros, err)
			continue
		}
		if err := restaurar(abs, estado); err != nil {
			erros = append(erros, fmt.Errorf("restaurar %s: %w", rel, err))
		}
	}
	for _, pasta := range foto.pastasNovas {
		if err := os.Remove(pasta); err != nil && !ausente(err) {
			erros = append(erros, fmt.Errorf("remover pasta criada pelo lote: %w", err))
		}
	}
	return errors.Join(erros...)
}

func restaurar(abs string, estado estadoDoArquivo) error {
	if !estado.existia {
		if err := os.Remove(abs); err != nil && !ausente(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	return escreverArquivoAtomico(abs, estado.conteudo, 0o644)
}

// ---- problemas da árvore ---------------------------------------------------------

// errosDaArvore são os problemas de nível erro da árvore inteira, com Onde completo. Uma
// árvore que não carrega vira um problema só, para o antes e o depois serem comparáveis.
func (c *Catalogo) errosDaArvore() []Problema {
	raizes, err := c.Tree()
	if err != nil {
		return []Problema{{Nivel: "erro", Onde: DirAgentes, Texto: "a árvore de agentes não carrega"}}
	}
	return errosDe(raizes)
}

func errosDe(agentes []*Agent) []Problema {
	var out []Problema
	for _, a := range agentes {
		for _, p := range a.Problemas {
			if p.Nivel == "erro" {
				p.Onde = a.Rel + "/" + p.Onde
				out = append(out, p)
			}
		}
		out = append(out, errosDe(a.Subagentes)...)
	}
	return out
}

// problemasNovos são os de depois que não estavam antes: o lote não responde por defeito que o catálogo já tinha.
func problemasNovos(depois, antes []Problema) []Problema {
	return slices.DeleteFunc(slices.Clone(depois), func(p Problema) bool {
		return slices.ContainsFunc(antes, func(a Problema) bool { return a.Onde == p.Onde && a.Texto == p.Texto })
	})
}
