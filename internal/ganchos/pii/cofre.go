package pii

import (
	"log/slog"
	"maps"
	"sync"
)

// Cofre guarda, só em memória e só durante o turno, o valor real de cada marcador.
// Nunca é persistido nem logado: String, GoString, MarshalJSON e LogValue escondem tudo.
type Cofre struct {
	mu      sync.Mutex
	valores map[string]string
}

const cofreRedigido = "Cofre(redigido)"

func NovoCofre() *Cofre { return &Cofre{valores: map[string]string{}} }

// Guardar acrescenta os achados de um Mascarar ao cofre.
func (c *Cofre) Guardar(achados map[string]string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	maps.Copy(c.valores, achados)
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
