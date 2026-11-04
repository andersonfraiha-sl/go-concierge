package entrada

type Contato struct {
	Phone string  `json:"phone"`
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

type Mensagem struct {
	ID         string
	ConversaID string
	Tenant     string
	// CompanyID: a company do Gruvi, quando o canal a conhece (o emulador a preenche
	// pelo condomínio escolhido); junto com o Tenant, resolve o condomínio.
	CompanyID string
	Contato   Contato
	Canal     map[string]string
	Origem    Origem
	Destino   Destino
	Tipo      string
	Texto     string
	Autor     string
	Revisao   uint64
}

type Origem struct {
	Conector  string `json:"conector"`
	Meio      string `json:"meio"`
	ConexaoID string `json:"conexao_id"`
}

type Destino struct {
	Conector  string            `json:"conector"`
	Meio      string            `json:"meio"`
	ConexaoID string            `json:"conexao_id"`
	Protocolo string            `json:"protocolo"`
	Telefone  string            `json:"telefone"`
	Canal     map[string]string `json:"canal,omitempty"`
	SessaoID  string            `json:"sessao_id,omitempty"`
}
