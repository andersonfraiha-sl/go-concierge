package sessoes

import (
	"go-concierge/internal/harness/persistencia"
	dominio "go-concierge/internal/harness/sessoes"
)

type RegistroSessao = dominio.RegistroSessao
type SessaoStore = dominio.SessaoStore
type SessaoEmMemoria = dominio.SessaoEmMemoria

var ComEscopoSessao = dominio.ComEscopoSessao
var ValidarEscopoSessao = dominio.ValidarEscopoSessao
var ValidarRegistroSessao = dominio.ValidarRegistroSessao
var NovaSessaoEmMemoria = dominio.NovaSessaoEmMemoria
var EscopoSessaoDoContexto = dominio.EscopoSessaoDoContexto
var ErrEscopoSessao = dominio.ErrEscopoSessao
var ErrRegistroSessao = dominio.ErrRegistroSessao
var ErrGravacaoNaoAplicada = persistencia.ErrGravacaoNaoAplicada
