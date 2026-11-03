package atende

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"go-concierge/internal/admin"
	"go-concierge/internal/okf"
)

// mapeamentoIndisponivel falha na leitura; as escritas não são usadas pelo atendimento.
type mapeamentoIndisponivel struct{ admin.Repositorio }

func (mapeamentoIndisponivel) Ler(context.Context) (admin.Mapeamento, error) {
	return admin.Mapeamento{}, errors.New("banco fora")
}

// Sem conseguir ler o mapeamento, o atendimento não sabe o que a administradora desligou:
// nenhum caso de uso fica disponível (falha fechada).
func TestMontarSemMapeamentoNegaTodosOsCasos(t *testing.T) {
	cat, err := okf.NewCatalogo(filepath.Join(raizDoRepo(t), "okf"))
	if err != nil {
		t.Fatal(err)
	}
	svc := &Servico{Cat: cat, Agente: "agentes/concierge", Sessoes: NewSessoes(), Mapeamento: mapeamentoIndisponivel{}}

	cfg, _, _, err := svc.montar(t.Context(), &Sessao{ID: "s1"})

	if err != nil {
		t.Fatal(err)
	}
	if cfg.Casos == nil {
		t.Fatal("sem mapeamento, Casos nil libera todos os casos")
	}
	for _, caso := range []struct{ nome, casoID string }{{"regimento", ""}, {"reserva", "reserva_area_comum"}} {
		if cfg.Casos(caso.nome, caso.casoID) {
			t.Fatalf("caso %s liberado sem mapeamento", caso.nome)
		}
	}
}
