package fixos

import (
	"maps"
	"reflect"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/pii"
)

const (
	nomePIIEntrada       = "pii_na_entrada"
	nomePIIResultado     = "pii_no_resultado"
	nomePIIResposta      = "pii_na_resposta"
	nomeTransbordoSemPII = "transbordo_sem_pii"
	motivoCofreInvalido  = "cofre do turno inválido"
)

// Saneamento devolve os fixos que tiram PII do que sai do turno e devolvem o valor real
// só na resposta ao morador. Os ganchos são puros: o evento não carrega um *pii.Cofre,
// só o mapa de marcadores. Cada gancho abre um Cofre a partir desse mapa (a numeração
// continua dali), mascara por ele e devolve o mapa novo no evento ajustado, sem mexer
// no recebido.
func Saneamento() ganchos.Fixos {
	return ganchos.Fixos{
		MensagemRecebida: ganchos.Pontos[ganchos.MensagemRecebida]{
			Saneamento: []ganchos.Gancho[ganchos.MensagemRecebida]{{Nome: nomePIIEntrada, Avaliar: piiNaEntrada}},
		},
		DepoisDaFerramenta: ganchos.Pontos[ganchos.DepoisDaFerramenta]{
			Saneamento: []ganchos.Gancho[ganchos.DepoisDaFerramenta]{{Nome: nomePIIResultado, Avaliar: piiNoResultado}},
		},
		AntesDeResponder: ganchos.Pontos[ganchos.AntesDeResponder]{
			Saneamento: []ganchos.Gancho[ganchos.AntesDeResponder]{{Nome: nomePIIResposta, Avaliar: piiNaResposta}},
		},
		TransbordoOuEncerramento: ganchos.Pontos[ganchos.TransbordoOuEncerramento]{
			Saneamento: []ganchos.Gancho[ganchos.TransbordoOuEncerramento]{
				{Nome: nomeTransbordoSemPII, Falha: ganchos.Aberta, Avaliar: transbordoSemPII},
			},
		},
	}
}

// piiNaEntrada neutraliza antes de mascarar: marcador digitado pelo morador perde a forma.
// Texto neutralizado sem achado também é ajuste, senão o Avaliar descartaria a troca.
func piiNaEntrada(e ganchos.MensagemRecebida) (ganchos.MensagemRecebida, ganchos.Decisao) {
	cofre, err := cofreDe(e.Cofre)
	if err != nil {
		return e, bloqueioDeCofre()
	}
	texto := cofre.Mascarar(pii.Neutralizar(e.Texto))
	if texto == e.Texto {
		return e, ganchos.Decisao{Tipo: ganchos.Deixar}
	}
	novos := novosMarcadores(e.Cofre, cofre.Ler())
	e.Texto, e.Cofre, e.Achados = texto, cofre.Ler(), juntar(e.Achados, novos)
	return e, ajuste(len(novos))
}

func piiNoResultado(e ganchos.DepoisDaFerramenta) (ganchos.DepoisDaFerramenta, ganchos.Decisao) {
	cofre, err := cofreDe(e.Cofre)
	if err != nil {
		return e, bloqueioDeCofre()
	}
	resultado := cofre.MascararEmValor(e.Resultado)
	if reflect.DeepEqual(resultado, e.Resultado) {
		return e, ganchos.Decisao{Tipo: ganchos.Deixar}
	}
	novos := novosMarcadores(e.Cofre, cofre.Ler())
	e.Resultado, e.Cofre, e.Achados = resultado, cofre.Ler(), juntar(e.Achados, novos)
	return e, ajuste(len(novos))
}

// piiNaResposta devolve só marcadores do cofre deste turno; marcador alheio fica como
// está e CPF solto sai «cpf». Telefone solto passa (spec §15).
func piiNaResposta(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
	texto := pii.Devolver(e.Texto, e.Cofre)
	if texto == e.Texto {
		return e, ganchos.Decisao{Tipo: ganchos.Deixar}
	}
	e.Texto = texto
	return e, ajuste(1)
}

// transbordoSemPII mascara o motivo com um cofre descartável: o motivo vai para gente e
// para persistência, onde ninguém devolve o valor.
func transbordoSemPII(e ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao) {
	cofre := pii.NovoCofre()
	motivo := cofre.Mascarar(e.Motivo)
	if motivo == e.Motivo {
		return e, ganchos.Decisao{Tipo: ganchos.Deixar}
	}
	e.Motivo = motivo
	return e, ajuste(len(cofre.Ler()))
}

func cofreDe(guardado map[string]string) (*pii.Cofre, error) {
	cofre := pii.NovoCofre()
	return cofre, cofre.Guardar(guardado)
}

func novosMarcadores(antes, depois map[string]string) map[string]string {
	novos := maps.Clone(depois)
	for m := range antes {
		delete(novos, m)
	}
	return novos
}

func juntar(base, extra map[string]string) map[string]string {
	juntos := make(map[string]string, len(base)+len(extra))
	maps.Copy(juntos, base)
	maps.Copy(juntos, extra)
	return juntos
}

func ajuste(quantidade int) ganchos.Decisao {
	return ganchos.Decisao{Tipo: ganchos.Ajustar, Quantidade: quantidade}
}

func bloqueioDeCofre() ganchos.Decisao {
	return ganchos.Decisao{Tipo: ganchos.Bloquear, Motivo: motivoCofreInvalido}
}
