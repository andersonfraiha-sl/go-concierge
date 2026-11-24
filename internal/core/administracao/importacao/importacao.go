package importacao

import (
	"context"
	"fmt"

	admin "go-concierge/internal/harness/configuracao"
	"go-concierge/internal/harness/memoria"
	"go-concierge/internal/harness/pedidos"
)

// Áreas do Relatorio.
const (
	AreaMapeamento = "mapeamento"
	AreaMemorias   = "memorias"
	AreaPedidos    = "pedidos"
	// AvisoProtocolosRedesenhados conta os pedidos cujo protocolo já era de outro contato e
	// foi sorteado de novo na gravação. Não é uma área: Lidos e Gravados valem o mesmo.
	AvisoProtocolosRedesenhados = "aviso_protocolos_redesenhados"
)

// Contagem de uma área: itens lidos dos arquivos e itens gravados no destino. Uma área que
// falha antes de gravar volta zerada.
type Contagem struct{ Lidos, Gravados int }

// Relatorio é a contagem por área.
type Relatorio map[string]Contagem

// Completo diz se toda área gravou tudo o que leu.
func (r Relatorio) Completo() bool {
	for _, area := range []string{AreaMapeamento, AreaMemorias, AreaPedidos} {
		if c := r[area]; c.Lidos != c.Gravados {
			return false
		}
	}
	return true
}

// Destinos são os repositórios que recebem a importação.
type Destinos struct {
	Mapeamento admin.Repositorio
	Memorias   memoria.Store
	Pedidos    pedidos.Store
	// Aviso, se informado, recebe cada ocorrência que não falha a importação (um protocolo
	// redesenhado por colisão entre administradoras, "P-ANTIGO → P-NOVO").
	Aviso func(string)
}

func Importar(ctx context.Context, dir string, destino Destinos) (Relatorio, error) {
	return Relatorio{}, fmt.Errorf("importação de registros legados indisponível: o banco v2 inicia vazio e exige company_id Nexus e condo_id ERP explícitos; configure canais e casos por SQL/seed")
}
