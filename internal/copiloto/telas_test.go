package copiloto

import (
	"slices"
	"testing"
)

func TestOperacoesDaTela(t *testing.T) {
	casos := []struct {
		tela Tela
		quer []TipoDeOperacao
	}{
		{TelaSkills, []TipoDeOperacao{CriarSkill, ExcluirSkill, PreencherTextos}},
		{TelaSkill, []TipoDeOperacao{PreencherTextos}},
		{TelaFluxo, []TipoDeOperacao{TrocarFluxo, PreencherTextos}},
		{TelaCondominios, []TipoDeOperacao{HabilitarSkill, Orientar}},
		{TelaCondominio, []TipoDeOperacao{HabilitarSkill, Orientar, Ajustar}},
		{TelaConcierge, []TipoDeOperacao{EditarPersona}},
		{TelaFerramentas, nil},
		{TelaAvancado, []TipoDeOperacao{EditarArquivo}},
		{Tela("desconhecida"), nil},
	}
	for _, c := range casos {
		t.Run(string(c.tela), func(t *testing.T) {
			if got := OperacoesDaTela(c.tela); !slices.Equal(got, c.quer) {
				t.Errorf("OperacoesDaTela(%q) = %v, quer %v", c.tela, got, c.quer)
			}
		})
	}
}

func TestOperacoesDaTelaDevolveCopia(t *testing.T) {
	ops := OperacoesDaTela(TelaSkills)
	ops[0] = Orientar
	if OperacoesDaTela(TelaSkills)[0] != CriarSkill {
		t.Error("alterar o retorno mudou as operações da tela")
	}
}
