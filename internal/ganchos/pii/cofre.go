package pii

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"regexp"
	"strconv"
	"sync"
)

// Cofre guarda, só em memória e só durante o turno, o valor real de cada marcador.
// Nunca é persistido nem logado: String, GoString, MarshalJSON e LogValue escondem
// tudo. Use sempre *Cofre (copiar o valor copiaria a trava e o mapa).
type Cofre struct {
	mu      sync.Mutex
	valores map[string]string
	proximo int
}

const cofreRedigido = "Cofre(redigido)"

var (
	ErrMarcadorInvalido = errors.New("marcador inválido")
	ErrMarcadorOcupado  = errors.New("marcador já guarda outro valor")

	reMarcadorInteiro = regexp.MustCompile(`^«(?:cpf|tel):(\d+)»$`)
)

func NovoCofre() *Cofre { return &Cofre{valores: map[string]string{}, proximo: 1} }

// Mascarar troca CPF e telefone do texto por marcadores e guarda os valores, numerando
// e guardando sob a mesma trava: chamadas em paralelo nunca dividem um marcador.
func (c *Cofre) Mascarar(texto string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	mascarado, achados := mascarar(texto, c.proximo)
	c.registrar(achados)
	return mascarado
}

// MascararEmValor é o Mascarar para resultado de ferramenta (map[string]any, []any).
// Não muda o original.
func (c *Cofre) MascararEmValor(v any) any {
	c.mu.Lock()
	defer c.mu.Unlock()
	mascarado, achados := mascararEmValor(v, c.proximo)
	c.registrar(achados)
	return mascarado
}

// Guardar acrescenta marcadores já prontos. Recusa marcador fora do formato e
// marcador que já guarda outro valor: nunca sobrescreve.
func (c *Cofre) Guardar(achados map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for m, v := range achados {
		if !reMarcadorInteiro.MatchString(m) {
			return fmt.Errorf("guardar no cofre: %w", ErrMarcadorInvalido)
		}
		if atual, ok := c.valores[m]; ok && atual != v {
			return fmt.Errorf("guardar no cofre: %w", ErrMarcadorOcupado)
		}
	}
	c.registrar(achados)
	return nil
}

// registrar grava os achados e avança a numeração além do maior marcador visto.
// Chamado com a trava.
func (c *Cofre) registrar(achados map[string]string) {
	for m, v := range achados {
		c.valores[m] = v
		if n := numeroDoMarcador(m); n >= c.proximo {
			c.proximo = n + 1
		}
	}
}

func numeroDoMarcador(m string) int {
	partes := reMarcadorInteiro.FindStringSubmatch(m)
	if partes == nil {
		return 0
	}
	n, err := strconv.Atoi(partes[1])
	if err != nil {
		return 0
	}
	return n
}

// Ler devolve uma cópia: quem lê não muda o cofre.
func (c *Cofre) Ler() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return maps.Clone(c.valores)
}

func (c *Cofre) String() string               { return cofreRedigido }
func (c *Cofre) GoString() string             { return cofreRedigido }
func (c *Cofre) MarshalJSON() ([]byte, error) { return []byte(`"` + cofreRedigido + `"`), nil }
func (c *Cofre) LogValue() slog.Value         { return slog.StringValue(cofreRedigido) }
