package catalogo

import "go-concierge/internal/harness/conteudo"

const Orquestrador = conteudo.Orquestrador
const Operador = conteudo.Operador
const DirAgentes = conteudo.DirAgentes
const AgentYAML = conteudo.AgentYAML
const PersonaMD = conteudo.PersonaMD
const CasoYAML = conteudo.CasoYAML

type Arquivo = conteudo.Arquivo
type Agent = conteudo.Agent
type Problema = conteudo.Problema

var stringList = conteudo.StringList
var StripFrontmatter = conteudo.StripFrontmatter
var splitFrontmatter = conteudo.SplitFrontmatter
var ErrSemFluxo = conteudo.ErrSemFluxo
var ErrSkillNaoExiste = conteudo.ErrSkillNaoExiste
var ErrVersaoDesatualizada = conteudo.ErrVersaoDesatualizada
var ErrNomeInvalido = conteudo.ErrNomeInvalido
var ErrSkillJaExiste = conteudo.ErrSkillJaExiste
var ErrTituloVazio = conteudo.ErrTituloVazio
var ErrVisibilidadeInvalida = conteudo.ErrVisibilidadeInvalida

type Textos = conteudo.Textos

var TextosNoCaso = conteudo.TextosNoCaso
var chavesDoCaso = conteudo.ChavesDoCaso

type Caso = conteudo.Caso
type CasoParam = conteudo.CasoParam

var ParseCaso = conteudo.ParseCaso

const LimiteDoResumo = conteudo.LimiteDoResumo

var ResumoDe = conteudo.ResumoDe

const nivelErro = conteudo.NivelErro

type ProblemaFluxo = conteudo.ProblemaFluxo

var ValidarFluxo = conteudo.ValidarFluxo
var problemasDeBlocos = conteudo.ProblemasDeBlocos
var Reclassificar = conteudo.Reclassificar
var FerramentasDesconhecidas = conteudo.FerramentasDesconhecidas
var EscritaPeloNome = conteudo.EscritaPeloNome

type Doc = conteudo.Doc
type Termo = conteudo.Termo
type Conhecimento = conteudo.Conhecimento

const KindAPI = conteudo.KindAPI
const KindDatabase = conteudo.KindDatabase
const KindKnowledge = conteudo.KindKnowledge
const KindMemory = conteudo.KindMemory
const KindAgent = conteudo.KindAgent
const KindAction = conteudo.KindAction
const KindActivity = conteudo.KindActivity
const KindForm = conteudo.KindForm
const OrigemMCP = conteudo.OrigemMCP

type Recurso = conteudo.Recurso
type Definicao = conteudo.Definicao
type Conexao = conteudo.Conexao
type SaidaCampo = conteudo.SaidaCampo
type ParamMeta = conteudo.ParamMeta
type RecursoDeclarado = conteudo.RecursoDeclarado

const passosIntro = conteudo.PassosIntro

var GerarPrincipal = conteudo.GerarPrincipal
var colapsar = conteudo.Colapsar

type Procedimento = conteudo.Procedimento
type SkillEfetiva = conteudo.SkillEfetiva
type TipoBloco = conteudo.TipoBloco

const BlocoBuscar = conteudo.BlocoBuscar
const BlocoGravar = conteudo.BlocoGravar
const BlocoEscolha = conteudo.BlocoEscolha
const BlocoPerguntar = conteudo.BlocoPerguntar
const BlocoFormulario = conteudo.BlocoFormulario
const BlocoConfirmar = conteudo.BlocoConfirmar
const BlocoMontarTexto = conteudo.BlocoMontarTexto
const BlocoReaproveitar = conteudo.BlocoReaproveitar
const BlocoResponder = conteudo.BlocoResponder

type CampoValor = conteudo.CampoValor
type Bloco = conteudo.Bloco
type Opcao = conteudo.Opcao
type Informacao = conteudo.Informacao
type Fluxo = conteudo.Fluxo

const IDDaResposta = conteudo.IDDaResposta

var FluxoDeYAML = conteudo.FluxoDeYAML
var AplicarFluxo = conteudo.AplicarFluxo
var substituirChaves = conteudo.SubstituirChaves
var codificarCaso = conteudo.CodificarCaso

type par = conteudo.Par
type seq = conteudo.Seq
type lista = conteudo.Lista

var noDe = conteudo.NoDe
var FixarIDsNoCaso = conteudo.FixarIDsNoCaso
