// Package memoria é a memória relacional entre o concierge e cada contato
// (telefone): fatos duráveis que os agentes vão capturando sobre a pessoa ao longo das
// conversas ("só tem um apartamento", "prefere ser chamado de Van", "tem um cachorro").
// Tudo o que um agente aprende sobre o morador fica aqui, comum à família inteira, e
// volta ao prompt em toda conversa seguinte daquele telefone.
//
// Hoje o armazenamento é um JSON por contato (okf/var/memorias/<account>/<telefone>.json);
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
)

// Fato é uma coisa durável sobre a pessoa, identificada por uma chave estável
// (unidades, nome_preferido, pets, veiculo, horario_preferido…). A mesma chave
// substitui o fato anterior.
type Fato struct {
	Chave     string    `json:"chave"`
	Texto     string    `json:"texto"`
	Fonte     string    `json:"fonte,omitempty"` // agente e protocolo que capturou
	Quando    time.Time `json:"quando"`
	Confianca float64   `json:"confianca,omitempty"`
}

// Memoria é o que o concierge sabe de um contato.
type Memoria struct {
	Account      string    `json:"account"`
	Telefone     string    `json:"telefone"`
	Nome         string    `json:"nome,omitempty"`
	Fatos        []Fato    `json:"fatos"`
	AtualizadaEm time.Time `json:"atualizada_em"`
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

var naoDigitos = regexp.MustCompile(`\D`)

// NormalizarTelefone: só dígitos.
func NormalizarTelefone(t string) string { return naoDigitos.ReplaceAllString(t, "") }

// Store guarda memórias por account e telefone.
type Store interface {
	Ler(ctx context.Context, account, telefone string) (Memoria, error)
	Gravar(ctx context.Context, m Memoria) error
	Listar(ctx context.Context, account string) ([]Memoria, error)
}

// Arquivos é o Store em JSON: <Dir>/<account>/<telefone>.json.
type Arquivos struct {
	Dir string
	mu  sync.Mutex
}

func NewArquivos(dir string) *Arquivos { return &Arquivos{Dir: dir} }

func (a *Arquivos) caminho(account, telefone string) (string, error) {
	tel := NormalizarTelefone(telefone)
	if tel == "" {
		return "", errors.New("memoria: telefone vazio")
	}
	acc := NormalizarChave(account)
	if acc == "" {
		acc = "default"
	}
	return filepath.Join(a.Dir, acc, tel+".json"), nil
}

func (a *Arquivos) Ler(_ context.Context, account, telefone string) (Memoria, error) {
	m := Memoria{Account: account, Telefone: NormalizarTelefone(telefone), Fatos: []Fato{}}
	p, err := a.caminho(account, telefone)
	if err != nil {
		return m, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return m, err
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("memoria: %s: %w", p, err)
	}
	if m.Fatos == nil {
		m.Fatos = []Fato{}
	}
	return m, nil
}

func (a *Arquivos) Gravar(_ context.Context, m Memoria) error {
	p, err := a.caminho(m.Account, m.Telefone)
	if err != nil {
		return err
	}
	m.Telefone = NormalizarTelefone(m.Telefone)
	if m.AtualizadaEm.IsZero() {
		m.AtualizadaEm = time.Now()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(m, "", "  ")
	return os.WriteFile(p, raw, 0o644)
}

func (a *Arquivos) Listar(_ context.Context, account string) ([]Memoria, error) {
	acc := NormalizarChave(account)
	if acc == "" {
		acc = "default"
	}
	entradas, err := os.ReadDir(filepath.Join(a.Dir, acc))
	if err != nil {
		if os.IsNotExist(err) {
			return []Memoria{}, nil
		}
		return nil, err
	}
	out := []Memoria{}
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(a.Dir, acc, e.Name()))
		if err != nil {
			continue
		}
		var m Memoria
		if json.Unmarshal(raw, &m) == nil {
			out = append(out, m)
		}
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
