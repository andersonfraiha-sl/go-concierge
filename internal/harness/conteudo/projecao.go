package conteudo

// Procedimento é um workflow além do principal: um .md solto ou uma pasta com etapas.
type Procedimento struct {
	Nome   string   `json:"nome"`
	Resumo string   `json:"resumo"`
	Etapas []string `json:"etapas"`
}
