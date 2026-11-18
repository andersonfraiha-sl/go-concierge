package auditoria

import "go-concierge/internal/ganchos"

var decisoesDoBanco = map[string]string{
	string(ganchos.Deixar):    "ALLOW",
	string(ganchos.Bloquear):  "BLOCK",
	string(ganchos.Confirmar): "CONFIRM",
	string(ganchos.Ajustar):   "ADJUST",
	decisaoFalhou:             "FAILED",
}
var origensDoBanco = map[string]string{ganchos.OrigemConcierge: "CONCIERGE", ganchos.OrigemCopiloto: "COPILOT"}

func valorNoBanco(v string, valores map[string]string) string {
	if x, ok := valores[v]; ok {
		return x
	}
	return v
}
func valorDoBanco(v string, valores map[string]string) string {
	for dominio, persistido := range valores {
		if persistido == v {
			return dominio
		}
	}
	return v
}
