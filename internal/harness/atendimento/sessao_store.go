package atendimento

import "go-concierge/internal/harness/sessoes"

type RegistroSessao = sessoes.RegistroSessao
type SessaoStore = sessoes.SessaoStore
type SessaoEmMemoria = sessoes.SessaoEmMemoria

var ComEscopoSessao = sessoes.ComEscopoSessao
var ValidarEscopoSessao = sessoes.ValidarEscopoSessao
var ValidarRegistroSessao = sessoes.ValidarRegistroSessao
var NovaSessaoEmMemoria = sessoes.NovaSessaoEmMemoria
var EscopoSessaoDoContexto = sessoes.EscopoSessaoDoContexto
var ErrEscopoSessao = sessoes.ErrEscopoSessao
var ErrRegistroSessao = sessoes.ErrRegistroSessao

var clonarRegistro = sessoes.ClonarRegistro
