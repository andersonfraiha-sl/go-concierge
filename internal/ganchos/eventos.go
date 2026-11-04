package ganchos

import (
	"time"

	"go-concierge/internal/admin"
	"go-concierge/internal/escopo"
	"go-concierge/internal/ganchos/pii"
)

// NomeDoEvento identifica o ponto do turno (ou do Copiloto) em que os ganchos rodam.
type NomeDoEvento string

const (
	EventoMensagemRecebida         NomeDoEvento = "mensagem_recebida"
	EventoAntesDaFerramenta        NomeDoEvento = "antes_da_ferramenta"
	EventoDepoisDaFerramenta       NomeDoEvento = "depois_da_ferramenta"
	EventoAntesDeResponder         NomeDoEvento = "antes_de_responder"
	EventoTransbordoOuEncerramento NomeDoEvento = "transbordo_ou_encerramento"
	EventoCopilotoAntesDeAceitar   NomeDoEvento = "copiloto_antes_de_aceitar"
)

// Ocorrencia é o evento que um gancho lê: uma struct por ponto, com tudo que ele pode ler.
type Ocorrencia interface{ Nome() NomeDoEvento }

// Identidade do morador no turno.
type Identidade struct {
	Cadastrado bool
	Papeis     []string
}

// MensagemRecebida dispara antes da camada 0 da cascata. Condominio chega vazio e é
// preenchido pelo fixo escopo_da_conversa; as regras saem dele.
type MensagemRecebida struct {
	Agora                    time.Time
	Texto, CompanyID, Tenant string
	Mapeamento               admin.Mapeamento
	Implantacao              escopo.Implantacao
	ContatoDaSessao          escopo.Contato
	Condominio               admin.Condominio
	Identidade               Identidade
	Cofre                    *pii.Cofre
}

// AntesDaFerramenta dispara no invocador único, antes de a ferramenta rodar.
type AntesDaFerramenta struct {
	Agora                    time.Time
	// Ferramenta é o nome MCP resolvido (decide o contexto B2C); Pedida é o nome que o
	// modelo chamou (entra nos motivos de recusa). Diferem no alias unidades.listar.
	Ferramenta, Pedida, Caso string
	Params, ContextoB2C      map[string]any
	Declarada, Permitida     bool
	Subagente                bool
	MotivoRecusa             string
	Permitidas               []string
	MCPObrigatoria           bool
	MCPDisponivel            bool
	Escrita, PeloMotorDoCaso bool
	Previa                   bool
	Confirmacao              string
	AdministradoraID         string
	CondominioID             string
	Identidade               Identidade
	Cofre                    *pii.Cofre
}

// DepoisDaFerramenta dispara no mesmo invocador, antes de o resultado voltar.
type DepoisDaFerramenta struct {
	Agora            time.Time
	Ferramenta, Caso string
	Params           map[string]any
	Resultado        any // payload dinâmico do Nexus, lido por caminho de campo
	AdministradoraID string
	CondominioID     string
	Identidade       Identidade
	Cofre            *pii.Cofre
}

// AntesDeResponder dispara logo antes do envio, venha a resposta de onde vier.
type AntesDeResponder struct {
	Agora            time.Time
	Texto            string
	AdministradoraID string
	CondominioID     string
	Identidade       Identidade
	Cofre            *pii.Cofre
}

// TransbordoOuEncerramento dispara ao transbordar ou encerrar a conversa.
type TransbordoOuEncerramento struct {
	Agora            time.Time
	Motivo           string
	Encerramento     bool
	AdministradoraID string
	CondominioID     string
	Identidade       Identidade
}

func (MensagemRecebida) Nome() NomeDoEvento         { return EventoMensagemRecebida }
func (AntesDaFerramenta) Nome() NomeDoEvento        { return EventoAntesDaFerramenta }
func (DepoisDaFerramenta) Nome() NomeDoEvento       { return EventoDepoisDaFerramenta }
func (AntesDeResponder) Nome() NomeDoEvento         { return EventoAntesDeResponder }
func (TransbordoOuEncerramento) Nome() NomeDoEvento { return EventoTransbordoOuEncerramento }

// Grava diz se a ferramenta escreve: decide a política FechadaSeGrava.
func (e AntesDaFerramenta) Grava() bool { return e.Escrita }

func (e MensagemRecebida) alvo() (condominioID, administradoraID string) {
	return e.Condominio.ID, e.Condominio.Administradora
}
func (e AntesDaFerramenta) alvo() (string, string)  { return e.CondominioID, e.AdministradoraID }
func (e DepoisDaFerramenta) alvo() (string, string) { return e.CondominioID, e.AdministradoraID }
func (e AntesDeResponder) alvo() (string, string)   { return e.CondominioID, e.AdministradoraID }
func (e TransbordoOuEncerramento) alvo() (string, string) {
	return e.CondominioID, e.AdministradoraID
}
