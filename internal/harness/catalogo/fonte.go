package catalogo

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"go-concierge/internal/harness/conteudo"
	"go-concierge/internal/harness/provedores/mcp"
)

type Leitor interface {
	CatalogoDoEscopo(context.Context, string, *int) (Retrato, error)
}

type FerramentasMCP interface {
	ListTools(context.Context) ([]mcp.Tool, error)
}

type Fonte struct {
	Dados Leitor
	MCP   FerramentasMCP
}

func (f *Fonte) Carregar(ctx context.Context, empresa string, condo *int) (conteudo.Catalogo, error) {
	if f == nil || f.Dados == nil {
		return nil, fmt.Errorf("catálogo: fonte de dados ausente")
	}
	retrato, err := f.Dados.CatalogoDoEscopo(ctx, empresa, condo)
	if err != nil {
		return nil, err
	}
	var recursos []conteudo.Recurso
	conferir := false
	vistos := map[string]bool{}
	adicionar := func(lista []conteudo.Recurso) {
		for _, item := range lista {
			if !vistos[item.Name] {
				vistos[item.Name] = true
				recursos = append(recursos, item)
			}
		}
	}
	if f.MCP != nil {
		conferir = true
		ctxMCP, cancelar := context.WithTimeout(ctx, 15*time.Second)
		tools, err := f.MCP.ListTools(ctxMCP)
		cancelar()
		if err != nil {
			slog.WarnContext(ctx, "catálogo MCP indisponível", "operacao", "catalogo_mcp", "erro", err)
		} else {
			adicionar(mcp.Recursos(tools))
		}
	}
	if retrato.RecursosNexus != nil {
		conferir = true
		adicionar(*retrato.RecursosNexus)
	} else if retrato.Conta != "" {
		if raw, existe := retrato.Arquivos["nexus/"+retrato.Conta+".yaml"]; existe {
			conferir = true
			lista, err := conteudo.InterpretarRecursosFixture(raw)
			if err != nil {
				return nil, fmt.Errorf("interpretar recursos locais: %w", err)
			}
			adicionar(lista)
		}
	}
	sort.Slice(recursos, func(i, j int) bool { return recursos[i].Name < recursos[j].Name })
	cat, err := Novo(retrato.Arquivos, recursos, conferir)
	if err != nil {
		return nil, err
	}
	dir, err := cat.ResolveAgente(retrato.Agente)
	if err != nil {
		return nil, err
	}
	if _, err := cat.Load(dir); err != nil {
		return nil, err
	}
	return cat, nil
}
