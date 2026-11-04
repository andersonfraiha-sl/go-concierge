import type { QueryKey } from '@tanstack/react-query';
import { type Eu, type Permissao, pode, podeNoCatalogo } from '@/acesso/eu';
import { CAMINHO_PERSONA, salvarArquivo } from '@/catalogo/contratos';
import {
  type Escopo,
  type Estado,
  habilitacaoDe,
  type Orientacao,
  salvarAjustes,
  salvarHabilitacao,
  salvarOrientacao,
} from '@/condominios/contratos';
import { ApiError } from '@/shared/api/http';
import {
  buscarFluxo,
  buscarTextos,
  chaveDoFluxo,
  chaveDosTextos,
  criarSkill,
  excluirSkill,
  paraIdentificador,
  salvarFluxo,
  salvarTextos,
  type Textos,
} from '@/skills/contratos';
import type { Operacao, TipoDeOperacao } from './contratos';

/** alvos: ids de realce (`useRealceDoCopiloto`); invalidar: consultas que a gravação mudou. */
export type Aplicacao = { alvos: string[]; invalidar: QueryKey[] };
/** A pessoa não confirmou: nada foi gravado. */
export const DESISTIU = 'desistiu';
export type Resultado = Aplicacao | typeof DESISTIU;

export type Ambiente = {
  escopo: Escopo;
  lerEstado: () => Estado | undefined;
  confirmar: (texto: string) => Promise<boolean>;
};

type OperacaoDo<T extends TipoDeOperacao> = Extract<Operacao, { tipo: T }>;
type Aplicador<T extends TipoDeOperacao> = (
  op: OperacaoDo<T>,
  ambiente: Ambiente,
) => Promise<Resultado>;
type TextosParciais = OperacaoDo<'preencher_textos'>['textos'];

const MENSAGEM_DE_VERSAO = 'Esta skill mudou desde a proposta. Peça de novo ao Copiloto.';
const MENSAGEM_SEM_ESTADO = 'Abra o condomínio de novo e aplique outra vez.';
const CHAVE_DAS_SKILLS: QueryKey = ['skills'];
const CHAVE_DO_ESTADO: QueryKey = ['estado-condominios'];
// Sem agente na proposta, o servidor usa o padrão; o front não sabe a chave dele, então refaz todas.
const CHAVES_DE_ARQUIVO: QueryKey[] = [['agente'], ['arquivo']];
const ORIENTACAO_VAZIA: Orientacao = { geral: '', por_caso: {} };

export const APLICADORES: { [T in TipoDeOperacao]: Aplicador<T> } = {
  preencher_textos: async (op) => {
    await gravarTextos(op.skill, op.textos);
    return {
      alvos: camposPreenchidos(op.textos).map((campo) => `skill:${op.skill}:${campo}`),
      invalidar: [chaveDosTextos(op.skill), CHAVE_DAS_SKILLS],
    };
  },
  trocar_fluxo: async (op) => {
    const lido = await buscarFluxo(op.skill);
    await semConflito(() => salvarFluxo(op.skill, { versao: lido.versao, fluxo: op.fluxo }));
    return { alvos: [`skill:${op.skill}:fluxo`], invalidar: [chaveDoFluxo(op.skill)] };
  },
  criar_skill: async (op) => {
    const { nome } = await criarSkill({ nome: paraIdentificador(op.titulo), titulo: op.titulo });
    if (op.textos) await gravarTextos(nome, op.textos);
    return { alvos: [`skill:${nome}`], invalidar: [CHAVE_DAS_SKILLS] };
  },
  excluir_skill: async (op, { confirmar }) => {
    const { textos } = await buscarTextos(op.skill);
    const titulo = textos.titulo || op.skill;
    const confirmou = await confirmar(`Excluir "${titulo}"? O fluxo e as regras somem junto.`);
    if (!confirmou) return DESISTIU;
    await excluirSkill(op.skill);
    return { alvos: [], invalidar: [CHAVE_DAS_SKILLS] };
  },
  habilitar_skill: async (op, ambiente) => {
    const { mapeamento } = estadoDe(ambiente);
    const { habilitacao } = habilitacaoDe(mapeamento, op.condominio, op.skill);
    const papeis = op.papeis ?? habilitacao.papeis;
    await salvarHabilitacao(
      {
        condominio: op.condominio,
        caso: op.skill,
        habilitado: op.habilitado,
        papeis,
        limpar: false,
      },
      ambiente.escopo,
    );
    return doCondominio(`condominio:${op.condominio}:skill:${op.skill}`);
  },
  orientar: async (op, ambiente) => {
    const atual = estadoDe(ambiente).mapeamento.orientacoes[op.condominio] ?? ORIENTACAO_VAZIA;
    const orientacao = orientacaoMesclada(atual, op.skill, op.texto);
    await salvarOrientacao({ condominio: op.condominio, ...orientacao }, ambiente.escopo);
    const onde = op.skill === undefined ? '' : `:skill:${op.skill}`;
    return doCondominio(`condominio:${op.condominio}${onde}:orientacao`);
  },
  ajustar: async (op, ambiente) => {
    const atuais = estadoDe(ambiente).mapeamento.payloads[op.condominio] ?? {};
    const ajustes = { ...atuais, [op.recurso]: { ...atuais[op.recurso], ...op.campos } };
    await salvarAjustes({ condominio: op.condominio, ajustes }, ambiente.escopo);
    return doCondominio(`condominio:${op.condominio}:ajustes:${op.recurso}`);
  },
  editar_persona: (op) => gravarArquivo(op.agente, CAMINHO_PERSONA, op.texto),
  editar_arquivo: (op) => gravarArquivo(op.agente, op.caminho, op.conteudo),
};

export function aplicarOperacao(op: Operacao, ambiente: Ambiente): Promise<Resultado> {
  return aplicarComo(op.tipo, op, ambiente);
}

// O tipo vem separado para o TypeScript casar a operação com o aplicador do mesmo tipo.
function aplicarComo<T extends TipoDeOperacao>(
  tipo: T,
  op: OperacaoDo<T>,
  ambiente: Ambiente,
): Promise<Resultado> {
  const aplicador: Aplicador<T> = APLICADORES[tipo];
  return aplicador(op, ambiente);
}

type Alcance = 'catalogo' | 'condominio';

// Espelho de `regras` em internal/copiloto/operacoes.go: o servidor já filtra, o cartão confere de novo.
const REGRAS: Record<TipoDeOperacao, { permissao: Permissao; alcance: Alcance }> = {
  preencher_textos: { permissao: 'skills.editar_textos', alcance: 'catalogo' },
  trocar_fluxo: { permissao: 'skills.editar_fluxo', alcance: 'catalogo' },
  criar_skill: { permissao: 'skills.criar', alcance: 'catalogo' },
  excluir_skill: { permissao: 'skills.excluir', alcance: 'catalogo' },
  habilitar_skill: { permissao: 'condominios.habilitar', alcance: 'condominio' },
  orientar: { permissao: 'condominios.habilitar', alcance: 'condominio' },
  ajustar: { permissao: 'condominios.habilitar', alcance: 'condominio' },
  editar_persona: { permissao: 'skills.editar_textos', alcance: 'catalogo' },
  editar_arquivo: { permissao: 'studio.avancado', alcance: 'catalogo' },
};

const PODE_NO_ALCANCE: Record<Alcance, (eu: Eu, permissao: Permissao) => boolean> = {
  catalogo: podeNoCatalogo,
  condominio: pode,
};

export function podeAplicar(eu: Eu, op: Operacao): boolean {
  const { permissao, alcance } = REGRAS[op.tipo];
  return PODE_NO_ALCANCE[alcance](eu, permissao);
}

async function gravarTextos(nome: string, novos: TextosParciais): Promise<void> {
  const lido = await buscarTextos(nome);
  const textos = mesclarTextos(lido.textos, novos);
  await semConflito(() => salvarTextos(nome, { versao: lido.versao, textos }));
}

const mesclarTextos = (atuais: Textos, novos: TextosParciais): Textos => ({
  ...atuais,
  titulo: novos.titulo ?? atuais.titulo,
  resumo: novos.resumo ?? atuais.resumo,
  quandoUsar: novos.quandoUsar ?? atuais.quandoUsar,
  palavrasChave: novos.palavrasChave ?? atuais.palavrasChave,
  frasesDeExemplo: novos.frasesDeExemplo ?? atuais.frasesDeExemplo,
});

const camposPreenchidos = (novos: TextosParciais): string[] =>
  Object.entries(novos)
    .filter(([, valor]) => valor !== undefined)
    .map(([campo]) => campo);

/** 409 é versão velha: a proposta foi feita sobre um texto que já mudou. */
async function semConflito<R>(gravar: () => Promise<R>): Promise<R> {
  try {
    return await gravar();
  } catch (erro) {
    if (erro instanceof ApiError && erro.status === 409) {
      throw new Error(MENSAGEM_DE_VERSAO, { cause: erro });
    }
    throw erro;
  }
}

function estadoDe(ambiente: Ambiente): Estado {
  const estado = ambiente.lerEstado();
  if (!estado) throw new Error(MENSAGEM_SEM_ESTADO);
  return estado;
}

/** Orientação de uma skill mantém a geral; a geral mantém as das skills, como nas telas. */
function orientacaoMesclada(
  atual: Orientacao,
  skill: string | undefined,
  texto: string,
): Orientacao {
  if (skill === undefined) return { geral: texto, por_caso: atual.por_caso };
  const porCaso = Object.fromEntries(
    Object.entries({ ...atual.por_caso, [skill]: texto }).filter(([, t]) => t.trim() !== ''),
  );
  return { geral: atual.geral, por_caso: porCaso };
}

const doCondominio = (alvo: string): Aplicacao => ({
  alvos: [alvo],
  invalidar: [CHAVE_DO_ESTADO],
});

async function gravarArquivo(
  agente: string | undefined,
  caminho: string,
  conteudo: string,
): Promise<Aplicacao> {
  const nome = agente ?? '';
  await salvarArquivo({ agente: nome, caminho, conteudo });
  return { alvos: [`arquivo:${nome}:${caminho}`], invalidar: CHAVES_DE_ARQUIVO };
}
