package acesso

import dominio "go-concierge/internal/acesso"

type Permissao = dominio.Permissao

const SkillsLer = dominio.SkillsLer
const SkillsEditarTextos = dominio.SkillsEditarTextos
const SkillsEditarFluxo = dominio.SkillsEditarFluxo
const SkillsCriar = dominio.SkillsCriar
const SkillsExcluir = dominio.SkillsExcluir
const SkillsEditarEtapas = dominio.SkillsEditarEtapas
const SkillsDecidirAtualizacao = dominio.SkillsDecidirAtualizacao
const SkillsPublicarBase = dominio.SkillsPublicarBase
const FerramentasEditar = dominio.FerramentasEditar
const CondominiosHabilitar = dominio.CondominiosHabilitar
const CondominiosEditar = dominio.CondominiosEditar
const ConversasVer = dominio.ConversasVer
const ConversasAtender = dominio.ConversasAtender
const PedidosVer = dominio.PedidosVer
const PedidosEditar = dominio.PedidosEditar
const MemoriaVer = dominio.MemoriaVer
const MemoriaEsquecer = dominio.MemoriaEsquecer
const AdministradorasEditar = dominio.AdministradorasEditar
const UsuariosEditar = dominio.UsuariosEditar
const StudioAvancado = dominio.StudioAvancado
const GanchosEditar = dominio.GanchosEditar
const GanchosHistorico = dominio.GanchosHistorico

type TipoEscopo = dominio.TipoEscopo

const TipoSuperlogica = dominio.TipoSuperlogica
const TipoAdministradora = dominio.TipoAdministradora
const TipoCondominio = dominio.TipoCondominio

type Escopo = dominio.Escopo

var EscopoCondominio = dominio.EscopoCondominio
var CondominioDoEscopo = dominio.CondominioDoEscopo

type Papel = dominio.Papel
type Atribuicao = dominio.Atribuicao
type Usuario = dominio.Usuario
type Concessao = dominio.Concessao
type Acesso = dominio.Acesso

var ErrCicloHeranca = dominio.ErrCicloHeranca
var PapeisIniciais = dominio.PapeisIniciais
var Permitido = dominio.Permitido
var PermitidoEmAlgum = dominio.PermitidoEmAlgum
var PermissoesEmAlgum = dominio.PermissoesEmAlgum
var PapelPrincipal = dominio.PapelPrincipal
var ValidarHeranca = dominio.ValidarHeranca
