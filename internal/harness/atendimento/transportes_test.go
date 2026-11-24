package atendimento

import (
	atendeapi "go-concierge/internal/core/api/atende"
	canaiscore "go-concierge/internal/core/clientes/canais"
)

type ATENDESender = canaiscore.ATENDESender
type ATENDETransferidor = canaiscore.ATENDETransferidor
type SenderPorCanal = canaiscore.SenderPorCanal
type TransferidorPorCanal = canaiscore.TransferidorPorCanal

var FormatarMensagem = canaiscore.FormatarMensagem
var DividirMensagem = canaiscore.DividirMensagem

type TipoEvento = atendeapi.TipoEvento
type Evento = atendeapi.Evento
type Dados = atendeapi.Dados
type Protocolo = atendeapi.Protocolo
type Contato = atendeapi.Contato
type Canal = atendeapi.Canal
type Criacao = atendeapi.Criacao
type Mensagem = atendeapi.Mensagem

var Parse = atendeapi.Parse

const EventoIgnorado = atendeapi.EventoIgnorado
const EventoElegivel = atendeapi.EventoElegivel
const EventoHumano = atendeapi.EventoHumano
const EventoFechado = atendeapi.EventoFechado

const EventoMensagem = atendeapi.EventoMensagem
const EventoMensagemAtendente = atendeapi.EventoMensagemAtendente
const EventoMensagemLegada = atendeapi.EventoMensagemLegada
const EventoProtocoloCriado = atendeapi.EventoProtocoloCriado
const EventoProtocoloFechadoAtendente = atendeapi.EventoProtocoloFechadoAtendente
const EventoProtocoloFechadoBot = atendeapi.EventoProtocoloFechadoBot
const EventoProtocoloFechadoSistema = atendeapi.EventoProtocoloFechadoSistema
