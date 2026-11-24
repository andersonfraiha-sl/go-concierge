package admin

import dominio "go-concierge/internal/harness/configuracao"

const Padrao = dominio.Padrao

type Administradora = dominio.Administradora
type Condominio = dominio.Condominio
type Orientacao = dominio.Orientacao
type Habilitacao = dominio.Habilitacao
type Mapeamento = dominio.Mapeamento

var ErrCondominioDesconhecido = dominio.ErrCondominioDesconhecido
var ErrForaDaImplantacao = dominio.ErrForaDaImplantacao
var ErrCondominioAmbiguo = dominio.ErrCondominioAmbiguo
var ErrCondominioIncompleto = dominio.ErrCondominioIncompleto
var ResolverCondominio = dominio.ResolverCondominio
var Contexto = dominio.Contexto
var ContextoDoCondominio = dominio.ContextoDoCondominio

type EmMemoria = dominio.EmMemoria

var NovaEmMemoria = dominio.NovaEmMemoria
var NovaEmMemoriaCom = dominio.NovaEmMemoriaCom
var ErrInvalido = dominio.ErrInvalido
var ErrJaExiste = dominio.ErrJaExiste
var ErrNaoEncontrado = dominio.ErrNaoEncontrado

type Chaves = dominio.Chaves

var ErrConflitoConcorrente = dominio.ErrConflitoConcorrente

type Repositorio = dominio.Repositorio
type Alvo = dominio.Alvo
