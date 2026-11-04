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

const (
	cofreRedigido = "Cofre(redigido)"
	// Um turno não chega perto disso; número maior em Guardar é lixo ou ataque e
	// levaria a numeração a estourar.
	maxNumeroDeMarcador = 1_000_000
)

var (
	ErrMarcadorInvalido = errors.New("marcador inválido")
	ErrMarcadorOcupado  = errors.New("marcador já guarda outro valor")

	reMarcadorInteiro = regexp.MustCompile(`^«(?:cpf|tel):(\d+)»$`)
)

func NovoCofre() *Cofre { return &Cofre{valores: map[string]string{}, proximo: 1} }

// Mascarar devolve também quantos marcadores esta chamada criou (valor repetido no mesmo texto
// reusa o marcador e não conta). Troca CPF e telefone do texto por marcadores e guarda os valores. Achar
// e montar o texto rodam fora da trava; dentro, só a numeração e a guarda, juntas:
// chamadas em paralelo nunca dividem um marcador e uma não espera a busca da outra.
func (c *Cofre) Mascarar(texto string) (mascarado string, novos int) {
	p := planejar(texto)
	valores := distintos([]plano{p})
	return aplicar(p, indexar(valores, c.marcarEGuardar(valores))), len(valores)
}

// MascararEmValor é o Mascarar para resultado de ferramenta (map[string]any, []any).
// Neutraliza também as chaves dos mapas. Não muda o original.
func (c *Cofre) MascararEmValor(v any) (mascarado any, novos int) {
	v = neutralizarChaves(v)
	planos := planejarValor(v)
	valores := distintos(planos)
	return aplicarValor(v, planos, indexar(valores, c.marcarEGuardar(valores))), len(valores)
}

// marcarEGuardar é a seção crítica: numera a partir do próximo livre, grava no cofre
// e avança o contador.
func (c *Cofre) marcarEGuardar(valores []valorAchado) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.valores) == 0 {
		// O primeiro lote do turno costuma ser o maior: o mapa já nasce no tamanho.
		c.valores = make(map[string]string, len(valores))
	}
	marcas := marcar(valores, c.proximo, c.valores)
	c.proximo += len(valores)
	return marcas
}

// Guardar acrescenta marcadores já prontos. Recusa marcador fora do formato, com
// número acima de maxNumeroDeMarcador e que já guarda outro valor: nunca sobrescreve.
// Tudo é conferido antes de gravar qualquer coisa ou mexer na numeração.
func (c *Cofre) Guardar(achados map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	maior := 0
	for m, v := range achados {
		n, ok := numeroDoMarcador(m)
		if !ok || n > maxNumeroDeMarcador {
			return fmt.Errorf("guardar no cofre: %w", ErrMarcadorInvalido)
		}
		if atual, ok := c.valores[m]; ok && atual != v {
			return fmt.Errorf("guardar no cofre: %w", ErrMarcadorOcupado)
		}
		maior = max(maior, n)
	}
	maps.Copy(c.valores, achados)
	c.proximo = max(c.proximo, maior+1)
	return nil
}

// numeroDoMarcador lê o N de «cpf:N» ou «tel:N».
func numeroDoMarcador(m string) (int, bool) {
	partes := reMarcadorInteiro.FindStringSubmatch(m)
	if partes == nil {
		return 0, false
	}
	n, err := strconv.Atoi(partes[1])
	return n, err == nil
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
