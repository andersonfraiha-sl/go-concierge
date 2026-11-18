package pedidos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"go-concierge/internal/escopo"
)

type Fake struct {
	mu      sync.Mutex
	pedidos map[string]map[string][]byte
	donos   map[string]string
	sortear func() string
}

func NovoFake() *Fake {
	return &Fake{pedidos: map[string]map[string][]byte{}, donos: map[string]string{}, sortear: NovoProtocolo}
}

func (f *Fake) Ler(_ context.Context, c escopo.Contato) (Carteira, error) {
	if err := validarContato(c); err != nil {
		return Carteira{}, err
	}
	c = c.Canonico()
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.carteira(c)
}

func (f *Fake) Gravar(_ context.Context, c escopo.Contato, cart Carteira) error {
	if err := validarContato(c); err != nil {
		return err
	}
	c = c.Canonico()
	f.mu.Lock()
	defer f.mu.Unlock()
	pedidos := copiasDoContato(c, cart.Pedidos)
	for i := range pedidos {
		p := &pedidos[i]
		if p.CondoID == nil && p.CondominioID != "" {
			legado, err := escopo.NovoContato(c.CompanyID, p.CondominioID, c.Telefone)
			if err != nil {
				return err
			}
			p.CondoID = legado.CondoID
		}
		if !escopo.CondominioValido(p.CondoID) || !c.Permite(p.CondoID) {
			return errors.New("pedido fora do condomínio autorizado")
		}
		if raw, ok := f.pedidos[c.ChavePessoa()][p.Protocolo]; ok {
			var atual Pedido
			if err := json.Unmarshal(raw, &atual); err != nil {
				return err
			}
			if !escopo.MesmoCondominio(atual.CondoID, p.CondoID) {
				return errors.New("condomínio de pedido imutável")
			}
		}
	}
	pedidos, err := redesenharColisoes(pedidos, f.deOutros(c), f.sortear)
	if err != nil {
		return fmt.Errorf("gravar pedidos de %s: %w", c.Mascarado(), err)
	}
	novos := map[string][]byte{}
	for protocolo, raw := range f.pedidos[c.ChavePessoa()] {
		var atual Pedido
		if err := json.Unmarshal(raw, &atual); err != nil {
			return err
		}
		if !escopo.MesmoCondominio(atual.CondoID, c.CondoID) {
			novos[protocolo] = raw
		}
	}
	for _, p := range pedidos {
		raw, err := json.Marshal(p)
		if err != nil {
			return err
		}
		novos[p.Protocolo] = raw
	}
	for protocolo := range f.pedidos[c.ChavePessoa()] {
		delete(f.donos, protocolo)
	}
	for protocolo := range novos {
		f.donos[protocolo] = c.ChavePessoa()
	}
	f.pedidos[c.ChavePessoa()] = novos
	adotarProtocolos(cart.Pedidos, pedidos)
	return nil
}

func (f *Fake) deOutros(c escopo.Contato) deOutros {
	return func(protocolos []string) (map[string]bool, error) {
		out := map[string]bool{}
		for _, p := range protocolos {
			if dono, ok := f.donos[p]; ok && dono != c.ChavePessoa() {
				out[p] = true
			}
		}
		return out, nil
	}
}

func (f *Fake) Listar(_ context.Context, companyID string) ([]Carteira, error) {
	if err := validarAdministradora(companyID); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []Carteira{}
	for _, pedidos := range f.pedidos {
		for _, raw := range pedidos {
			var ped Pedido
			if err := json.Unmarshal(raw, &ped); err != nil {
				return nil, err
			}
			if ped.CompanyID == companyID {
				cart, err := f.carteira(escopo.Contato{CompanyID: companyID, Telefone: ped.Telefone})
				if err != nil {
					return nil, err
				}
				out = append(out, cart)
			}
			break
		}
	}
	slices.SortFunc(out, maisRecentesPrimeiro)
	return out, nil
}

func (f *Fake) carteira(c escopo.Contato) (Carteira, error) {
	pedidos := []*Pedido{}
	for _, raw := range f.pedidos[c.ChavePessoa()] {
		var p Pedido
		if err := json.Unmarshal(raw, &p); err != nil {
			return Carteira{}, err
		}
		if c.Permite(p.CondoID) {
			p.AdministradoraID = p.CompanyID
			pedidos = append(pedidos, &p)
		}
	}
	return montar(c, pedidos), nil
}
