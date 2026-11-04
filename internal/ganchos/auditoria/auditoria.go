// Package auditoria grava as decisões dos ganchos que mexeram no turno (bloquear, ajustar,
// confirmar ou falhar) e apaga o que passou da retenção. Nunca guarda texto da mensagem,
// params, resultado, CPF ou telefone: o Motivo é o do gancho.
package auditoria

import (
	"time"

	"go-concierge/internal/ganchos"
)

const (
	OrigemConcierge = "concierge"
	OrigemCopiloto  = "copiloto"

	decisaoFalhou = "falhou"
)

// Linha é uma linha de ganchos_auditoria. Base traz origem, ids e sessão; LinhasDe preenche o resto.
type Linha struct {
	CriadoEm                                                                         time.Time
	Origem                                                                           string // "concierge" | "copiloto"
	Evento, Gancho, RegraID                                                          string
	RegraVersao                                                                      int
	Decisao, Motivo                                                                  string
	AdministradoraID, CondominioID, SessaoID, Protocolo, UsuarioID, Tela, Ferramenta string
	Quantidade                                                                       int
}

// LinhasDe devolve uma linha por gancho que decidiu algo ou falhou. Deixar sem falha não
// gera linha (vira só métrica).
func LinhasDe(regs []ganchos.Registro, base Linha) []Linha {
	linhas := make([]Linha, 0, len(regs))
	for _, r := range regs {
		if r.Decisao.Tipo == ganchos.Deixar && !r.Falhou {
			continue
		}
		linhas = append(linhas, linhaDe(r, base))
	}
	return linhas
}

func linhaDe(r ganchos.Registro, base Linha) Linha {
	l := base
	l.Evento = string(r.Evento)
	l.Gancho = r.Decisao.Origem.Gancho
	l.RegraID = r.Decisao.Origem.RegraID
	l.RegraVersao = r.Decisao.Origem.RegraVersao
	l.Decisao = string(r.Decisao.Tipo)
	if r.Falhou {
		l.Decisao = decisaoFalhou
	}
	l.Motivo = r.Decisao.Motivo
	l.Quantidade = r.Decisao.Quantidade
	return l
}
