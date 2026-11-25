package camadas

import (
	"log/slog"

	camadasruntime "go-concierge/internal/harness/camadas"
	okf "go-concierge/internal/harness/conteudo"
)

type Efetivas = camadasruntime.Efetivas

func NovasEfetivas(log *slog.Logger) *Efetivas { return camadasruntime.NovasEfetivas(log) }
func Efetivar(casoYAML []byte, d Documento) (okf.SkillEfetiva, error) {
	return camadasruntime.Efetivar(casoYAML, d)
}
