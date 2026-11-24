package conteudo

type Catalogo interface {
	ResolveAgente(string) (string, error)
	Load(string) (*Agent, error)
	ProjecaoCom(*Agent, *string) string
	Roteador(*Agent) string
	Politicas(*Agent) string
	Indice(*Agent) []Arquivo
	Procedimentos(*Agent) []Procedimento
	RecursosNexus() []Recurso
	RecursosDoAgente(*Agent) []RecursoDeclarado
	LerDado(*Agent, string) ([]byte, error)
	Ler(*Agent, Arquivo) ([]byte, error)
	LerArquivo(string, string) ([]byte, error)
	Conhecimento(*Agent) Conhecimento
	Ancestrais(string) []string
}
