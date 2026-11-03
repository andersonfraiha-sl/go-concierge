// Package memoria é a memória relacional entre o concierge e cada contato
// (telefone): fatos duráveis que os agentes vão capturando sobre a pessoa ao longo das
// conversas ("só tem um apartamento", "prefere ser chamado de Van", "tem um cachorro").
// Tudo o que um agente aprende sobre o morador fica aqui, comum à família inteira, e
// volta ao prompt em toda conversa seguinte daquele telefone.
//
// Hoje o armazenamento é um JSON por contato (okf/var/memorias/<administradora>/<telefone>.json);
// a interface Store permite trocar por Postgres ou pelo recurso `memory` do Nexus.
package memoria

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"go-concierge/internal/escopo"
)

// Fato é uma coisa durável sobre a pessoa, identificada por uma chave estável
// (unidades, nome_preferido, pets, veiculo, horario_preferido…). A mesma chave
// substitui o fato anterior.
type Fato struct {
	Chave string `json:"chave"`
	Texto string `json:"texto"`
	Fonte string `json:"fonte,omitempty"` // agente e protocolo que capturou
	// CondominioID: o condomínio da conversa em que o fato foi capturado.
	CondominioID string    `json:"condominio_id,omitempty"`
	Quando       time.Time `json:"quando"`
	Confianca    float64   `json:"confianca,omitempty"`
}

// Memoria é o que o concierge sabe de um contato dentro de uma administradora.
type Memoria struct {
	AdministradoraID string    `json:"administradora_id"`
	Telefone         string    `json:"telefone"`
	Nome             string    `json:"nome,omitempty"`
	Fatos            []Fato    `json:"fatos"`
	AtualizadaEm     time.Time `json:"atualizada_em"`
}

// Upsert põe ou substitui um fato pela chave. Devolve true se algo mudou.
func (m *Memoria) Upsert(f Fato) bool {
	f.Chave = NormalizarChave(f.Chave)
	f.Texto = strings.TrimSpace(f.Texto)
	if f.Chave == "" || f.Texto == "" {
		return false
	}
	if f.Quando.IsZero() {
		f.Quando = time.Now()
	}
	for i, atual := range m.Fatos {
		if atual.Chave == f.Chave {
			if atual.Texto == f.Texto {
				return false
			}
			m.Fatos[i] = f
			m.AtualizadaEm = f.Quando
			return true
		}
	}
	m.Fatos = append(m.Fatos, f)
	m.AtualizadaEm = f.Quando
	return true
}

// Esquecer tira um fato pela chave.
func (m *Memoria) Esquecer(chave string) bool {
	chave = NormalizarChave(chave)
	for i, f := range m.Fatos {
		if f.Chave == chave {
			m.Fatos = append(m.Fatos[:i], m.Fatos[i+1:]...)
			m.AtualizadaEm = time.Now()
			return true
		}
	}
	return false
}

// Texto é como a memória entra no prompt.
func (m Memoria) Texto() string {
	if len(m.Fatos) == 0 {
		return ""
	}
	var b strings.Builder
	for _, f := range m.Fatos {
		fmt.Fprintf(&b, "- %s: %s", f.Chave, f.Texto)
		if !f.Quando.IsZero() {
			fmt.Fprintf(&b, " (em %s)", f.Quando.Format("02/01/2006"))
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

var reChave = regexp.MustCompile(`[^a-z0-9_]+`)

// NormalizarChave: minúsculas, sem acentos comuns, snake_case.
func NormalizarChave(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer("á", "a", "à", "a", "ã", "a", "â", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c", " ", "_", "-", "_")
	s = r.Replace(s)
	s = reChave.ReplaceAllString(s, "")
	return strings.Trim(s, "_")
}

// Store guarda memórias por contato (administradora e telefone).
type Store interface {
	Ler(ctx context.Context, c escopo.Contato) (Memoria, error)
	Gravar(ctx context.Context, c escopo.Contato, m Memoria) error
	Listar(ctx context.Context, administradoraID string) ([]Memoria, error)
}

// Arquivos é o Store em JSON: <Dir>/<administradora>/<telefone>.json.
type Arquivos struct {
	Dir string
	mu  sync.Mutex
}

func NewArquivos(dir string) *Arquivos { return &Arquivos{Dir: dir} }

var errContatoInvalido = errors.New("contato sem administradora válida ou sem telefone")

// reAdministradora é o formato de id do Admin; vale também como nome de pasta seguro.
var reAdministradora = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func (a *Arquivos) pasta(administradoraID string) (string, error) {
	if !reAdministradora.MatchString(administradoraID) {
		return "", fmt.Errorf("memória da administradora %q: %w", administradoraID, errContatoInvalido)
	}
	return filepath.Join(a.Dir, administradoraID), nil
}

func (a *Arquivos) caminho(c escopo.Contato) (string, error) {
	pasta, err := a.pasta(c.AdministradoraID)
	if err != nil {
		return "", err
	}
	if c.Telefone == "" {
		return "", errContatoInvalido
	}
	return filepath.Join(pasta, c.Telefone+".json"), nil
}

func (a *Arquivos) Ler(_ context.Context, c escopo.Contato) (Memoria, error) {
	p, err := a.caminho(c)
	if err != nil {
		return Memoria{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return lerArquivo(p, c)
}

func lerArquivo(p string, c escopo.Contato) (Memoria, error) {
	m := Memoria{AdministradoraID: c.AdministradoraID, Telefone: c.Telefone, Fatos: []Fato{}}
	raw, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, fmt.Errorf("ler memória %s: %w", p, err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("ler memória %s: %w", p, err)
	}
	if m.Fatos == nil {
		m.Fatos = []Fato{}
	}
	return m, nil
}

// Gravar guarda a memória sob o contato: a administradora e o telefone são os dele.
func (a *Arquivos) Gravar(_ context.Context, c escopo.Contato, m Memoria) error {
	p, err := a.caminho(c)
	if err != nil {
		return err
	}
	m.AdministradoraID, m.Telefone = c.AdministradoraID, c.Telefone
	if m.AtualizadaEm.IsZero() {
		m.AtualizadaEm = time.Now()
	}
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("codificar memória de %s: %w", c.Chave(), err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("gravar memória %s: %w", p, err)
	}
	if err := os.WriteFile(p, raw, 0o644); err != nil {
		return fmt.Errorf("gravar memória %s: %w", p, err)
	}
	return nil
}

func (a *Arquivos) Listar(_ context.Context, administradoraID string) ([]Memoria, error) {
	pasta, err := a.pasta(administradoraID)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	entradas, err := os.ReadDir(pasta)
	if errors.Is(err, os.ErrNotExist) {
		return []Memoria{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listar memórias %s: %w", pasta, err)
	}
	out := []Memoria{}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		c := escopo.Contato{AdministradoraID: administradoraID, Telefone: strings.TrimSuffix(e.Name(), ".json")}
		m, err := lerArquivo(filepath.Join(pasta, e.Name()), c)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AtualizadaEm.After(out[j].AtualizadaEm) })
	return out, nil
}

// Extrator decide, a partir de uma troca (mensagem do morador e resposta), quais
// fatos duráveis sobre a pessoa valem guardar. O harness fornece o extrator com
// modelo; sem modelo, as Regras abaixo pegam os casos óbvios.
type Extrator interface {
	Extrair(ctx context.Context, mensagem, resposta string, atual Memoria) ([]Fato, error)
}

// Regras é um extrator por padrões, para rodar sem modelo (mock e testes).
type Regras struct{}

var padroes = []struct {
	re    *regexp.Regexp
	chave string
	texto func(m []string) string
}{
	{regexp.MustCompile(`(?i)\b(s[oó] tenho|tenho s[oó]|tenho apenas|moro em)\s+(um|uma|1)\s+(ap(?:artamento|ê|to)?|unidade|casa|sala)\b`), "unidades", func(m []string) string { return "tem só uma unidade (" + strings.ToLower(m[3]) + ")" }},
	{regexp.MustCompile(`(?i)\b(?:pode|podem) me chamar de\s+([A-Za-zÀ-ú]+)`), "nome_preferido", func(m []string) string { return "prefere ser chamado de " + m[1] }},
	{regexp.MustCompile(`(?i)\btenho (um|uma|dois|duas|\d+)\s+(cachorros?|gatos?|pets?|animais?)\b`), "pets", func(m []string) string { return "tem " + strings.ToLower(m[1]+" "+m[2]) }},
}

func (Regras) Extrair(_ context.Context, mensagem, _ string, _ Memoria) ([]Fato, error) {
	var out []Fato
	for _, p := range padroes {
		if m := p.re.FindStringSubmatch(mensagem); m != nil {
			out = append(out, Fato{Chave: p.chave, Texto: p.texto(m), Confianca: 0.8})
		}
	}
	return out, nil
}
