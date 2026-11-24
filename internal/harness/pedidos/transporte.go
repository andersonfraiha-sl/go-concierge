package pedidos

import "go-concierge/internal/escopo"

func CarteiraParaTransporte(contato escopo.Contato, cart Carteira) (Carteira, error) {
	if err := contato.Validar(); err != nil {
		return Carteira{}, err
	}
	contato = contato.Canonico()
	cart.Pedidos = append([]*Pedido(nil), cart.Pedidos...)
	for i, p := range cart.Pedidos {
		if p == nil || p.CondoID != nil || p.CondominioID == "" {
			continue
		}
		alvo, err := escopo.NovoContato(contato.CompanyID, p.CondominioID, contato.Telefone)
		if err != nil {
			return Carteira{}, err
		}
		copia := *p
		copia.CondoID = alvo.CondoID
		cart.Pedidos[i] = &copia
	}
	return cart, nil
}
