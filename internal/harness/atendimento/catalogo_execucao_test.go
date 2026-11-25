package atendimento

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	corecatalogo "go-concierge/internal/core/catalogo"
	"go-concierge/internal/harness/catalogo"
	"go-concierge/internal/harness/conteudo"
)

type catalogoDadosTeste struct {
	retrato catalogo.Retrato
	empresa string
}

type fonteDePedidosIndisponivel struct{}

func (fonteDePedidosIndisponivel) Carregar(context.Context, string, *int) (conteudo.Catalogo, error) {
	return nil, errors.New("catálogo indisponível")
}

func TestPedidosManuaisNaoUsamCatalogoAntigoQuandoFonteFalha(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	sess := falar(t, svc, "pedido-antes-da-falha", "quero reservar o salão")
	protocolo := ultimoConcierge(sess).Pedidos[0].Protocolo
	svc.Catalogos = fonteDePedidosIndisponivel{}
	if _, _, err := svc.Carteira(t.Context(), contatoDeTeste(t, "adm-a")); err == nil {
		t.Fatal("carteira usou catálogo antigo após falha")
	}
	if _, _, _, err := svc.PreencherPedido(t.Context(), sess.ID, protocolo, "Ana", nil, false); err == nil {
		t.Fatal("preenchimento usou catálogo antigo após falha")
	}
}

func TestPedidosManuaisUsamContratoDoRetratoAtual(t *testing.T) {
	svc, _, _ := servicoComPedidos(t)
	sess := falar(t, svc, "pedido-inicial", "quero reservar o salão")
	protocolo := ultimoConcierge(sess).Pedidos[0].Protocolo
	arquivos, err := corecatalogo.LerDisco(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	retrato := catalogo.Retrato{Agente: "agentes/concierge", Conta: "cond-001", Arquivos: map[string][]byte{}}
	for _, arquivo := range arquivos {
		retrato.Arquivos[arquivo.Caminho] = arquivo.Conteudo
	}
	caminho := "agentes/concierge/agentes/reserva-area-comum/caso.yaml"
	caso := string(retrato.Arquivos[caminho])
	caso = strings.Replace(caso, "params:\n", "params:\n  - name: observacao\n    title: Observação atualizada\n    ask: Qual observação?\n", 1)
	caso = strings.Replace(caso, "- collect: [data]", "- collect: [data, observacao]", 1)
	retrato.Arquivos[caminho] = []byte(caso)
	svc.Catalogos = &catalogo.Fonte{Dados: &catalogoDadosTeste{retrato: retrato}}
	_, slots, err := svc.Carteira(t.Context(), contatoDeTeste(t, "adm-a"))
	if err != nil {
		t.Fatal(err)
	}
	achou := false
	for _, slot := range slots["reserva_area_comum"] {
		if slot.Nome == "observacao" {
			achou = true
		}
	}
	if !achou {
		t.Fatalf("novo campo não apareceu: %+v", slots["reserva_area_comum"])
	}
	_, p, _, err := svc.PreencherPedido(t.Context(), sess.ID, protocolo, "Ana", map[string]any{"observacao": "Ligar antes"}, false)
	if err != nil || p == nil || p.Valores["observacao"] != "Ligar antes" {
		t.Fatalf("preenchimento com contrato novo: pedido=%+v erro=%v", p, err)
	}
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
