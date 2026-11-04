import { type Eu, pode, podeNoCatalogo } from '@/acesso/eu';
import type { ContextoDoCopiloto, Tela, TipoDeOperacao } from './contratos';

type Alcance = 'catalogo' | 'condominio';

type Regra = {
  readonly permissao: Parameters<typeof pode>[1];
  readonly alcance: Alcance;
};

// Espelho de `regras` em internal/copiloto/operacoes.go. Uma sugestão só aparece quando o
// servidor aceitaria a operação que ela propõe — manter os dois em sincronia.
const REGRAS: Readonly<Record<TipoDeOperacao, Regra>> = {
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

function podeAplicar(eu: Eu, tipo: TipoDeOperacao): boolean {
  const regra = REGRAS[tipo];
  return regra.alcance === 'catalogo'
    ? podeNoCatalogo(eu, regra.permissao)
    : pode(eu, regra.permissao);
}

type Sugestao = {
  readonly texto: string;
  /** Ausente = pergunta, nunca uma mudança: sempre visível. Presente = a operação que o servidor validaria. */
  readonly operacao?: TipoDeOperacao;
};

const LIMITE_DE_SUGESTOES = 3;

const SUGESTOES_POR_TELA: Readonly<Record<Tela, readonly Sugestao[]>> = {
  skills: [
    { texto: 'Crie uma skill para reservar a quadra', operacao: 'criar_skill' },
    { texto: 'Quais skills gravam no sistema?' },
    { texto: 'Escreva o resumo das skills que estão sem', operacao: 'preencher_textos' },
  ],
  skill: [
    { texto: 'Melhore o resumo desta skill', operacao: 'preencher_textos' },
    { texto: 'Sugira frases de exemplo', operacao: 'preencher_textos' },
    { texto: 'Explique quando esta skill é usada' },
  ],
  fluxo: [
    { texto: 'Explique este fluxo passo a passo' },
    { texto: 'Adicione um Confirmar antes de gravar', operacao: 'trocar_fluxo' },
    { texto: 'Pergunte o horário antes de reservar', operacao: 'trocar_fluxo' },
  ],
  condominios: [
    { texto: 'Quais condomínios estão sem ids?' },
    { texto: 'Desligue a reserva de área comum onde não há área', operacao: 'habilitar_skill' },
  ],
  condominio: [
    { texto: 'O que muda deste condomínio para o Padrão?' },
    { texto: 'Escreva uma orientação sobre mudanças aos domingos', operacao: 'orientar' },
    { texto: 'Ajuste a antecedência da reserva para 3 dias', operacao: 'ajustar' },
  ],
  concierge: [
    { texto: 'Deixe a persona mais acolhedora', operacao: 'editar_persona' },
    { texto: 'Revise a persona para respostas curtas', operacao: 'editar_persona' },
  ],
  ferramentas: [
    { texto: 'Explique o que esta ferramenta faz' },
    { texto: 'Quais skills usam esta ferramenta?' },
  ],
  avancado: [
    { texto: 'Explique este arquivo' },
    { texto: 'Revise este arquivo e proponha melhorias', operacao: 'editar_arquivo' },
  ],
};

/** Até 3 sugestões para a tela do contexto, só o que `eu` pode de fato pedir. */
export function sugestoesPara(contexto: ContextoDoCopiloto, eu: Eu): string[] {
  return SUGESTOES_POR_TELA[contexto.tela]
    .filter((sugestao) => sugestao.operacao === undefined || podeAplicar(eu, sugestao.operacao))
    .slice(0, LIMITE_DE_SUGESTOES)
    .map((sugestao) => sugestao.texto);
}
