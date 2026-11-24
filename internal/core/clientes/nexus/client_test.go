package nexus

import "testing"

func TestDisponibilidade(t *testing.T) {
	var nulo *Client
	for _, caso := range []struct {
		nome   string
		client *Client
		quer   bool
	}{
		{nome: "nulo", client: nulo},
		{nome: "sem URL", client: New("", "")},
		{nome: "configurado", client: New("https://nexus.example", ""), quer: true},
	} {
		t.Run(caso.nome, func(t *testing.T) {
			if got := caso.client.Disponivel(); got != caso.quer {
				t.Fatalf("Disponivel() = %t; esperado %t", got, caso.quer)
			}
		})
	}
}
