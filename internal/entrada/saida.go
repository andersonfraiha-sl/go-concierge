package entrada

type ComandoSaida struct {
	Operacao string  `json:"operacao"`
	Destino  Destino `json:"destino"`
	Texto    string  `json:"texto,omitempty"`
	Motivo   string  `json:"motivo,omitempty"`
}

type ResultadoSaida struct {
	Resultado string   `json:"resultado"`
	Partes    []string `json:"partes,omitempty"`
	Erro      string   `json:"erro,omitempty"`
}

type EntregaIncerta struct{ Erro error }

func (e *EntregaIncerta) Error() string { return "resultado da entrega incerto: " + e.Erro.Error() }
func (e *EntregaIncerta) Unwrap() error { return e.Erro }
