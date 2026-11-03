package pedidos

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"

	"go-concierge/internal/escopo"
)

// Fake é o Store dos testes: guarda cada pedido serializado, como a coluna dados, e
// devolve cópias; o protocolo tem um dono só, como a chave primária.
type Fake struct {
	mu      sync.Mutex
	pedidos map[string]map[string][]byte // contato (escopo.Contato.Chave) → protocolo → JSON
	donos   map[string]string            // protocolo → contato
}

func NovoFake() *Fake {
	return &Fake{pedidos: map[string]map[string][]byte{}, donos: map[string]string{}}
}

func (f *Fake) Ler(_ context.Context, c escopo.Contato) (Carteira, error) {
	if err := validarContato(c); err != nil {
		return Carteira{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.carteira(c)
}

func (f *Fake) Gravar(_ context.Context, c escopo.Contato, cart Carteira) error {
	if err := validarContato(c); err != nil {
		return err
	}
	novos := map[string][]byte{}
	for _, p := range copiasDoContato(c, cart.Pedidos) {
		raw, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("codificar pedido %s de %s: %w", p.Protocolo, c.Mascarado(), err)
		}
		novos[p.Protocolo] = raw
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for protocolo := range novos {
		if dono, ok := f.donos[protocolo]; ok && dono != c.Chave() {
			return fmt.Errorf("gravar pedido %s de %s: %w", protocolo, c.Mascarado(), errProtocoloDeOutroContato)
		}
	}
	for protocolo := range f.pedidos[c.Chave()] {
		delete(f.donos, protocolo)
	}
	for protocolo := range novos {
		f.donos[protocolo] = c.Chave()
	}
	f.pedidos[c.Chave()] = novos
	return nil
}

func (f *Fake) Listar(_ context.Context, administradoraID string) ([]Carteira, error) {
	if err := validarAdministradora(administradoraID); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Carteira{}
	for chave, pedidos := range f.pedidos {
		telefone, ok := strings.CutPrefix(chave, administradoraID+":")
		if !ok || len(pedidos) == 0 {
			continue
		}
		cart, err := f.carteira(escopo.Contato{AdministradoraID: administradoraID, Telefone: telefone})
		if err != nil {
			return nil, err
		}
		out = append(out, cart)
	}
	slices.SortFunc(out, maisRecentesPrimeiro)
	return out, nil
}

// carteira decodifica os pedidos do contato; quem chama segura f.mu.
func (f *Fake) carteira(c escopo.Contato) (Carteira, error) {
	pedidos := []*Pedido{}
	for _, raw := range f.pedidos[c.Chave()] {
		var p Pedido
		if err := json.Unmarshal(raw, &p); err != nil {
			return Carteira{}, fmt.Errorf("ler pedidos de %s: %w", c.Mascarado(), err)
		}
		pedidos = append(pedidos, &p)
	}
	return montar(c, pedidos), nil
}
