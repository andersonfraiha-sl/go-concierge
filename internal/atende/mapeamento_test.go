package atende

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/escopo"
	"go-concierge/internal/okf"
)

// mapeamentoIndisponivel falha na leitura; as escritas não são usadas pelo atendimento.
type mapeamentoIndisponivel struct{ admin.Repositorio }

func (mapeamentoIndisponivel) Ler(context.Context) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, errors.New("banco fora")
}

// Sem conseguir ler o mapeamento, o atendimento não sabe o que a administradora desligou
// nem qual account usar: a sessão não é montada (falha fechada, com o erro propagado).
func TestMontarSemMapeamentoFalha(t *testing.T) {
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	svc := &Servico{Cat: cat, Agente: "agentes/concierge", Sessoes: NewSessoes(), Mapeamento: mapeamentoIndisponivel{}}

	_, _, _, err = svc.montar(t.Context(), &Sessao{ID: "s1"})

	if err == nil || !strings.Contains(err.Error(), "banco fora") {
		t.Fatalf("montar deveria falhar com o erro do mapeamento: %v", err)
	}
}

// Condomínio da sessão que sumiu do mapeamento: também não monta.
func TestMontarSemCondominioDaSessaoFalha(t *testing.T) {
	svc, _, _ := servicoMultiAdministradora(t, escopo.Implantacao{})
	sess := &Sessao{ID: "s1"}
	sess.fixarContato(escopo.Contato{AdministradoraID: "adm-a", CondominioID: "cond-removido", Telefone: telefoneMorador}, "5982")

	_, _, _, err := svc.montar(t.Context(), sess)

	if !errors.Is(err, admin.ErrCondominioDesconhecido) {
		t.Fatalf("montar deveria falhar sem o condomínio: %v", err)
	}
}
