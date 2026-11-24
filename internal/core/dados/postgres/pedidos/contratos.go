package pedidos

import dominio "go-concierge/internal/harness/pedidos"

type Slot = dominio.Slot
type Passo = dominio.Passo
type Contrato = dominio.Contrato

var ContratoDe = dominio.ContratoDe
var NoFusoSaoPaulo = dominio.NoFusoSaoPaulo
var Render = dominio.Render

const Coletando = dominio.Coletando
const Pronto = dominio.Pronto
const Confirmando = dominio.Confirmando
const Executando = dominio.Executando
const Concluido = dominio.Concluido
const ComAtendente = dominio.ComAtendente
const Cancelado = dominio.Cancelado
const Falhou = dominio.Falhou
const OrigemMorador = dominio.OrigemMorador
const OrigemAtendente = dominio.OrigemAtendente
const OrigemSistema = dominio.OrigemSistema
const OrigemMemoria = dominio.OrigemMemoria

type Pedido = dominio.Pedido
type Pendencia = dominio.Pendencia
type Confirmacao = dominio.Confirmacao
type Resultado = dominio.Resultado
type Origem = dominio.Origem
type Evento = dominio.Evento
type Opcao = dominio.Opcao
type Carteira = dominio.Carteira

var NovoProtocolo = dominio.NovoProtocolo

type Store = dominio.Store

var ErrProtocoloEmUso = dominio.ErrProtocoloEmUso

const tentativasProtocolo = dominio.TentativasProtocolo

type deOutros = dominio.DeOutros

var redesenharColisoes = dominio.RedesenharColisoes
var adotarProtocolos = dominio.AdotarProtocolos
var validarAdministradora = dominio.ValidarAdministradora
var validarContato = dominio.ValidarContato
var copiasDoContato = dominio.CopiasDoContato
var montar = dominio.Montar
var maisRecentesPrimeiro = dominio.MaisRecentesPrimeiro
var ResumoParaAtendente = dominio.ResumoParaAtendente
var CarteiraParaTransporte = dominio.CarteiraParaTransporte

type CasoResumo = dominio.CasoResumo
type Entrada = dominio.Entrada
type PedidoExtraido = dominio.PedidoExtraido
type Extracao = dominio.Extracao
type Confirmada = dominio.Confirmada
type Extrator = dominio.Extrator
type Regras = dominio.Regras

var Confirmado = dominio.Confirmado
var Negado = dominio.Negado

type Fake = dominio.Fake

var NovoFake = dominio.NovoFake
var NovoFakeComGerador = dominio.NovoFakeComGerador

type Ambiente = dominio.Ambiente

const CanceladoAPedido = dominio.CanceladoAPedido

type Previsor = dominio.Previsor
type Previsao = dominio.Previsao
type PerguntaDosGanchos = dominio.PerguntaDosGanchos
type Saida = dominio.Saida

var Avancar = dominio.Avancar
var TextoDaPergunta = dominio.TextoDaPergunta
var OpcoesDeDados = dominio.OpcoesDeDados

const AvisoAposVezes = dominio.AvisoAposVezes
const AtendenteAposVezes = dominio.AtendenteAposVezes

var Aplicar = dominio.Aplicar
var Confirmar = dominio.Confirmar
var Cancelar = dominio.Cancelar
var AoAtendente = dominio.AoAtendente
