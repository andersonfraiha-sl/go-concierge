package fixos

import (
	"reflect"
	"strings"

	"go-concierge/internal/ganchos"
	"go-concierge/internal/ganchos/pii"
)

const (
	nomePIIEntrada   = "pii_na_entrada"
	nomePIIResultado = "pii_no_resultado"
	nomePIIResposta  = "pii_na_resposta"
)

// Saneamento devolve os fixos que tiram PII do que sai do turno e devolvem o valor real
// só na resposta ao morador. Os ganchos são puros, com uma única mutação sancionada: o
// cofre do evento é o do turno (estado do turno, atômico, e o Turno é quem o possui), e
// mascarar grava nele. Fora isso, o evento recebido não muda.
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
				{Nome: ganchos.GanchoTransbordoSemPII, Falha: ganchos.Aberta, Avaliar: transbordoSemPII},
			},
		},
	}
}

// piiNaEntrada neutraliza antes de mascarar: marcador digitado pelo morador perde a forma.
// Texto neutralizado sem achado também é ajuste, senão o Avaliar descartaria a troca.
func piiNaEntrada(e ganchos.MensagemRecebida) (ganchos.MensagemRecebida, ganchos.Decisao) {
	texto, novos := e.Cofre.Mascarar(pii.Neutralizar(e.Texto))
	if texto == e.Texto {
		return e, ganchos.Decisao{Tipo: ganchos.Deixar}
	}
	e.Texto = texto
	return e, ajuste(novos)
}

func piiNoResultado(e ganchos.DepoisDaFerramenta) (ganchos.DepoisDaFerramenta, ganchos.Decisao) {
	resultado, novos := e.Cofre.MascararEmValor(e.Resultado)
	if reflect.DeepEqual(resultado, e.Resultado) {
		return e, ganchos.Decisao{Tipo: ganchos.Deixar}
	}
	e.Resultado = resultado
	return e, ajuste(novos)
}

// piiNaResposta devolve só marcadores do cofre deste turno; marcador alheio fica como
// está e CPF solto sai «cpf». Telefone solto passa (spec §15). Quantidade conta os
// marcadores devolvidos.
func piiNaResposta(e ganchos.AntesDeResponder) (ganchos.AntesDeResponder, ganchos.Decisao) {
	valores := e.Cofre.Ler()
	texto := pii.Devolver(e.Texto, valores)
	if texto == e.Texto {
		return e, ganchos.Decisao{Tipo: ganchos.Deixar}
	}
	devolvidos := 0
	for marcador := range valores {
		devolvidos += strings.Count(e.Texto, marcador)
	}
	e.Texto = texto
	return e, ajuste(devolvidos)
}

// transbordoSemPII mascara o motivo com um cofre descartável: o motivo vai para gente e
// para persistência, onde ninguém devolve o valor.
func transbordoSemPII(e ganchos.TransbordoOuEncerramento) (ganchos.TransbordoOuEncerramento, ganchos.Decisao) {
	cofre := pii.NovoCofre()
	motivo, novos := cofre.Mascarar(e.Motivo)
	if motivo == e.Motivo {
		return e, ganchos.Decisao{Tipo: ganchos.Deixar}
	}
	e.Motivo = motivo
	return e, ajuste(novos)
}

func ajuste(quantidade int) ganchos.Decisao {
	return ganchos.Decisao{Tipo: ganchos.Ajustar, Quantidade: quantidade}
}
