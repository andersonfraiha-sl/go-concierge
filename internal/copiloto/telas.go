package copiloto

import "slices"

// Tela é onde a pessoa está no Studio; decide o que o Copiloto pode propor ali.
type Tela string

const (
	TelaSkills      Tela = "skills"
	TelaSkill       Tela = "skill"
	TelaFluxo       Tela = "fluxo"
	TelaCondominios Tela = "condominios"
	TelaCondominio  Tela = "condominio"
	TelaConcierge   Tela = "concierge"
	TelaFerramentas Tela = "ferramentas"
	TelaAvancado    Tela = "avancado"
)

// Contexto é a tela aberta e o que está selecionado nela.
type Contexto struct {
	Tela       Tela   `json:"tela"`
	Skill      string `json:"skill,omitempty"`
	Condominio string `json:"condominio,omitempty"`
	Agente     string `json:"agente,omitempty"`
	Caminho    string `json:"caminho,omitempty"`
}

// operacoesPorTela: Ferramentas fica de fora de propósito, ali o Copiloto só explica.
var operacoesPorTela = map[Tela][]TipoDeOperacao{
	TelaSkills:      {CriarSkill, ExcluirSkill, PreencherTextos},
	TelaSkill:       {PreencherTextos},
	TelaFluxo:       {TrocarFluxo, PreencherTextos},
	TelaCondominios: {HabilitarSkill, Orientar},
	TelaCondominio:  {HabilitarSkill, Orientar, Ajustar},
	TelaConcierge:   {EditarPersona},
	TelaAvancado:    {EditarArquivo},
}

// OperacoesDaTela devolve o que o Copiloto pode propor na tela; nil quando nada.
func OperacoesDaTela(t Tela) []TipoDeOperacao {
	return slices.Clone(operacoesPorTela[t])
}

func telaFaz(t Tela, tipo TipoDeOperacao) bool {
	return slices.Contains(operacoesPorTela[t], tipo)
}
