import { type Eu, pode, podeNoCatalogo } from '@/acesso/eu';
import type { ContextoDoCopiloto, Tela } from './contratos';

type Sugestao = {
  readonly texto: string;
  /** Ausente = qualquer pessoa pode pedir (pergunta, nunca uma mudança). */
  readonly podeVer?: (eu: Eu) => boolean;
};

const LIMITE_DE_SUGESTOES = 3;

// Cada linha é uma pergunta (sempre visível) ou uma operação (só quem tem a permissão dela).
const SUGESTOES_POR_TELA: Readonly<Record<Tela, readonly Sugestao[]>> = {
  skills: [
    {
      texto: 'Crie uma skill para reservar a quadra',
      podeVer: (eu) => podeNoCatalogo(eu, 'skills.criar'),
    },
    { texto: 'Quais skills gravam no sistema?' },
    {
      texto: 'Escreva o resumo das skills que estão sem',
      podeVer: (eu) => podeNoCatalogo(eu, 'skills.editar_textos'),
    },
  ],
  skill: [
    {
      texto: 'Melhore o resumo desta skill',
      podeVer: (eu) => podeNoCatalogo(eu, 'skills.editar_textos'),
    },
    {
      texto: 'Sugira frases de exemplo',
      podeVer: (eu) => podeNoCatalogo(eu, 'skills.editar_textos'),
    },
    { texto: 'Explique quando esta skill é usada' },
  ],
  fluxo: [
    { texto: 'Explique este fluxo passo a passo' },
    {
      texto: 'Adicione um Confirmar antes de gravar',
      podeVer: (eu) => podeNoCatalogo(eu, 'skills.editar_fluxo'),
    },
    {
      texto: 'Pergunte o horário antes de reservar',
      podeVer: (eu) => podeNoCatalogo(eu, 'skills.editar_fluxo'),
    },
  ],
  condominios: [
    { texto: 'Quais condomínios estão sem ids?' },
    {
      texto: 'Desligue a reserva de área comum onde não há área',
      podeVer: (eu) => pode(eu, 'condominios.habilitar'),
    },
  ],
  condominio: [
    { texto: 'O que muda deste condomínio para o Padrão?' },
    {
      texto: 'Escreva uma orientação sobre mudanças aos domingos',
      podeVer: (eu) => pode(eu, 'condominios.editar'),
    },
    {
      texto: 'Ajuste a antecedência da reserva para 3 dias',
      podeVer: (eu) => pode(eu, 'condominios.editar'),
    },
  ],
  concierge: [
    {
      texto: 'Deixe a persona mais acolhedora',
      podeVer: (eu) => podeNoCatalogo(eu, 'skills.editar_textos'),
    },
    {
      texto: 'Revise a persona para respostas curtas',
      podeVer: (eu) => podeNoCatalogo(eu, 'skills.editar_textos'),
    },
  ],
  ferramentas: [
    { texto: 'Explique o que esta ferramenta faz' },
    { texto: 'Quais skills usam esta ferramenta?' },
  ],
  avancado: [
    { texto: 'Explique este arquivo' },
    {
      texto: 'Revise este arquivo e proponha melhorias',
      podeVer: (eu) => podeNoCatalogo(eu, 'studio.avancado'),
    },
  ],
};

/** Até 3 sugestões para a tela do contexto, só o que `eu` pode de fato pedir. */
export function sugestoesPara(contexto: ContextoDoCopiloto, eu: Eu): string[] {
  return SUGESTOES_POR_TELA[contexto.tela]
    .filter((sugestao) => sugestao.podeVer === undefined || sugestao.podeVer(eu))
    .slice(0, LIMITE_DE_SUGESTOES)
    .map((sugestao) => sugestao.texto);
}
