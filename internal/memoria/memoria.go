// Package memoria é a memória relacional entre o concierge e cada contato
// (telefone): preferências de atendimento de uma lista fechada ("prefere ser chamado de
// Van", "resposta curta") que o morador afirma ao longo das conversas, comuns à família
// inteira, e que voltam ao prompt em toda conversa seguinte daquele telefone. Nada fora
// da lista entra: o que um modelo extrai nunca vira identidade, vínculo ou autorização.
//
// O armazenamento é o Postgres, em duas tabelas: memoria_contatos (nome e quando mudou)
// e memoria_fatos (um fato por linha).
package memoria

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"go-concierge/internal/escopo"
)

// Fato é uma preferência durável sobre a pessoa, identificada por uma chave da lista
// fechada (ChavesDePreferencia). A mesma chave substitui o fato anterior.
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

// Upsert devolve a memória com a preferência posta ou substituída pela chave, e true se
// algo mudou. Só entra preferência da lista fechada (NormalizarPreferencia); o resto é
// recusado. O mesmo valor com Quando mais novo renova a data: reafirmar renova a
// preferência. Não altera o receptor: o fato sem Quando entra com o instante zero, quem
// chama decide o relógio.
func (m Memoria) Upsert(f Fato) (Memoria, bool) {
	f, ok := NormalizarPreferencia(f)
	if !ok {
		return m, false
	}
	i := slices.IndexFunc(m.Fatos, func(atual Fato) bool { return atual.Chave == f.Chave })
	if i >= 0 && m.Fatos[i].Texto == f.Texto && !f.Quando.After(m.Fatos[i].Quando) {
		return m, false
	}
	fatos := slices.Clone(m.Fatos)
	if i >= 0 {
		fatos[i] = f
	} else {
		fatos = append(fatos, f)
	}
	m.Fatos, m.AtualizadaEm = fatos, f.Quando
	return m, true
}

// Esquecer devolve a memória sem o fato da chave, e true se ele existia. Não altera o receptor.
func (m Memoria) Esquecer(chave string) (Memoria, bool) {
	chave = NormalizarChave(chave)
	ehDaChave := func(f Fato) bool { return f.Chave == chave }
	if !slices.ContainsFunc(m.Fatos, ehDaChave) {
		return m, false
	}
	m.Fatos = slices.DeleteFunc(slices.Clone(m.Fatos), ehDaChave)
	return m, true
}

// vazia é a memória de um contato de quem nada se sabe.
func vazia(c escopo.Contato) Memoria {
	return Memoria{AdministradoraID: c.AdministradoraID, Telefone: c.Telefone, Fatos: []Fato{}}
}

// comoGuardada é a memória como um Store a guarda: sob o contato, uma chave só por fato (a
// última vence, no lugar da primeira) e AtualizadaEm preenchida com agora quando vier vazia.
func comoGuardada(c escopo.Contato, m Memoria, agora time.Time) Memoria {
	m.AdministradoraID, m.Telefone = c.AdministradoraID, c.Telefone
	m.Fatos = semChavesRepetidas(m.Fatos)
	if m.AtualizadaEm.IsZero() {
		m.AtualizadaEm = agora
	}
	return m
}

func semChavesRepetidas(fatos []Fato) []Fato {
	out := make([]Fato, 0, len(fatos))
	posicao := map[string]int{}
	for _, f := range fatos {
		if i, repetida := posicao[f.Chave]; repetida {
			out[i] = f
			continue
		}
		posicao[f.Chave] = len(out)
		out = append(out, f)
	}
	return out
}

// maisRecentesPrimeiro ordena a listagem: a memória atualizada por último primeiro, telefone no empate.
func maisRecentesPrimeiro(a, b Memoria) int {
	if c := b.AtualizadaEm.Compare(a.AtualizadaEm); c != 0 {
		return c
	}
	return strings.Compare(a.Telefone, b.Telefone)
}

// validarContato: a administradora tem o formato do Admin e o telefone não é vazio.
func validarContato(c escopo.Contato) error {
	if !escopo.AdministradoraValida(c.AdministradoraID) || c.Telefone == "" {
		return errContatoInvalido
	}
	return nil
}

func validarAdministradora(administradoraID string) error {
	if !escopo.AdministradoraValida(administradoraID) {
		return fmt.Errorf("memória da administradora %q: %w", administradoraID, errContatoInvalido)
	}
	return nil
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

var errContatoInvalido = errors.New("contato sem administradora válida ou sem telefone")

// ErrRespostaForaDoFormato: o extrator com modelo recebeu algo que não é o JSON pedido.
var ErrRespostaForaDoFormato = errors.New("resposta do extrator fora do formato")

// Extrator decide, a partir de uma troca (mensagem do morador e resposta), quais
// fatos duráveis sobre a pessoa valem guardar. O harness fornece o extrator com
// modelo; sem modelo, as Regras abaixo pegam os casos óbvios.
type Extrator interface {
	Extrair(ctx context.Context, mensagem, resposta string, atual Memoria) ([]Fato, error)
}

// Regras é um extrator por padrões, para rodar sem modelo (mock e testes). Só extrai
// preferências da lista fechada.
type Regras struct{}

var padroes = []struct {
	re    *regexp.Regexp
	chave string
	valor func(m []string) string
}{
	{regexp.MustCompile(`(?i)\b(?:pode|podem) me chamar de\s+([A-Za-zÀ-ú]+)`), chaveNomePreferido, func(m []string) string { return m[1] }},
	{regexp.MustCompile(`(?i)\bpode ser (?:mais )?(?:curto|direto|resumido)\b`), chaveTamanhoResposta, func([]string) string { return "curta" }},
	{regexp.MustCompile(`(?i)\b(?:com|mais) detalhes?\b`), chaveTamanhoResposta, func([]string) string { return "detalhada" }},
}

func (Regras) Extrair(_ context.Context, mensagem, _ string, _ Memoria) ([]Fato, error) {
	var out []Fato
	for _, p := range padroes {
		m := p.re.FindStringSubmatch(mensagem)
		if m == nil {
			continue
		}
		if f, ok := NormalizarPreferencia(Fato{Chave: p.chave, Texto: p.valor(m), Confianca: 0.8}); ok {
			out = append(out, f)
		}
	}
	return out, nil
}
