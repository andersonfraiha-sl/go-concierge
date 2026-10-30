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
	Contato    Contato
	Canal      map[string]string
	Tipo       string
	Texto      string
}
