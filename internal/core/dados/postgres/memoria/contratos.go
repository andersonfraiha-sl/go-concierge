package memoria

import dominio "go-concierge/internal/harness/memoria"

type Fake = dominio.Fake

var NovoFake = dominio.NovoFake

type Fato = dominio.Fato
type Memoria = dominio.Memoria

var vazia = dominio.Vazia
var comoGuardada = dominio.ComoGuardada
var semChavesRepetidas = dominio.SemChavesRepetidas
var maisRecentesPrimeiro = dominio.MaisRecentesPrimeiro
var validarContato = dominio.ValidarContato
var validarAdministradora = dominio.ValidarAdministradora
var NormalizarChave = dominio.NormalizarChave

type Store = dominio.Store

var ErrRespostaForaDoFormato = dominio.ErrRespostaForaDoFormato

type Extrator = dominio.Extrator
type Regras = dominio.Regras

var preparar = dominio.Preparar
var ParaTransporte = dominio.ParaTransporte
