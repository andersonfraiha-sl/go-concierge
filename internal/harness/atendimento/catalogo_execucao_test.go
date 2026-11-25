package atendimento

import (
	"context"
	"path/filepath"
	"testing"

	corecatalogo "go-concierge/internal/core/catalogo"
	"go-concierge/internal/harness/catalogo"
)

type catalogoDadosTeste struct {
	retrato catalogo.Retrato
	empresa string
}

func (f *catalogoDadosTeste) CatalogoDoEscopo(_ context.Context, empresa string, _ *int) (catalogo.Retrato, error) {
	f.empresa = empresa
	return f.retrato, nil
}

func TestProcessarComCatalogoEmMemoriaSemDiretorioDeRuntime(t *testing.T) {
	arquivos, err := corecatalogo.LerDisco(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	retrato := catalogo.Retrato{Agente: "agentes/concierge", Conta: "cond-001", Arquivos: map[string][]byte{}}
	for _, arquivo := range arquivos {
		retrato.Arquivos[arquivo.Caminho] = arquivo.Conteudo
	}
	dados := &catalogoDadosTeste{retrato: retrato}
	svc, out, _ := servicoComPedidos(t)
	svc.Catalogos = &catalogo.Fonte{Dados: dados}
	svc.Cat = nil
	svc.Pedidos = nil
	svc.TelefoneB2C = ""
	svc.Config.MCP = nil
	msg := mensagemExemplo(t)
	sess, err := svc.Processar(t.Context(), msg)
	if err != nil {
		t.Fatal(err)
	}
	if dados.empresa == "" || sess == nil || len(out.Itens(msg.ConversaID)) == 0 {
		t.Fatalf("execução sem catálogo compartilhado falhou: empresa=%q", dados.empresa)
	}
}
