package ganchos

var eventosDoBanco = map[NomeDoEvento]string{
	EventoMensagemRecebida:         "MESSAGE_RECEIVED",
	EventoAntesDaFerramenta:        "BEFORE_TOOL",
	EventoDepoisDaFerramenta:       "AFTER_TOOL",
	EventoAntesDeResponder:         "BEFORE_RESPONSE",
	EventoTransbordoOuEncerramento: "HANDOFF_OR_CLOSE",
	EventoCopilotoAntesDeAceitar:   "COPILOT_BEFORE_ACCEPT",
}

func EventoNoBanco(v NomeDoEvento) string {
	if x, ok := eventosDoBanco[v]; ok {
		return x
	}
	return string(v)
}

func EventoDoBanco(v string) NomeDoEvento {
	for dominio, persistido := range eventosDoBanco {
		if persistido == v {
			return dominio
		}
	}
	return NomeDoEvento(v)
}
