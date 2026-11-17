import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { TELAS } from './contratos';
import { sugestoesPara } from './sugestoes';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: [
    'skills.ler',
    'skills.editar_textos',
    'skills.editar_fluxo',
    'skills.criar',
    'skills.excluir',
    'ferramentas.editar',
    'condominios.habilitar',
    'condominios.editar',
    'administradoras.editar',
    'studio.avancado',
  ],
  administradoraId: null,
  dev: false,
};

const gestor: Eu = {
  id: 'u2',
  nome: 'Beto',
  papel: 'gestor',
  permissoes: ['skills.ler'],
  administradoraId: 'adm-1',
  dev: false,
};

const SUGESTOES_PARA_CONSTRUTOR: Record<(typeof TELAS)[number], string[]> = {
  skills: [
    'Crie uma skill para reservar a quadra',
    'Quais skills gravam no sistema?',
    'Escreva o resumo das skills que estão sem',
  ],
  skill: [
    'Melhore o resumo desta skill',
    'Sugira frases de exemplo',
    'Explique quando esta skill é usada',
  ],
  fluxo: [
    'Explique este fluxo passo a passo',
    'Adicione um Confirmar antes de gravar',
    'Pergunte o horário antes de reservar',
  ],
  condominios: [
    'Quais condomínios estão sem ids?',
    'Desligue a reserva de área comum onde não há área',
  ],
  condominio: [
    'O que muda deste condomínio para o Padrão?',
    'Escreva uma orientação sobre mudanças aos domingos',
    'Ajuste a antecedência da reserva para 3 dias',
  ],
  concierge: ['Deixe a persona mais acolhedora', 'Revise a persona para respostas curtas'],
  ferramentas: ['Explique o que esta ferramenta faz', 'Quais skills usam esta ferramenta?'],
  avancado: ['Explique este arquivo', 'Revise este arquivo e proponha melhorias'],
};

describe('sugestoesPara', () => {
  it.each(TELAS)('suggestsByScreen(%s)', (tela) => {
    expect(sugestoesPara({ tela }, construtor)).toEqual(SUGESTOES_PARA_CONSTRUTOR[tela]);
  });

  it('limitsToThreeSuggestions', () => {
    for (const tela of TELAS) {
      expect(sugestoesPara({ tela }, construtor).length).toBeLessThanOrEqual(3);
    }
  });

  it('hidesSuggestionsGestorCannotApply', () => {
    expect(sugestoesPara({ tela: 'skills' }, gestor)).toEqual(['Quais skills gravam no sistema?']);
  });
});

type Caso = {
  readonly tela: (typeof TELAS)[number];
  readonly texto: string;
  readonly permissao: Eu['permissoes'][number];
  readonly alcance: 'catalogo' | 'condominio';
};

// Espelho independente de `regras` em internal/copiloto/operacoes.go: não reaproveita a tabela
// de sugestoes.ts, para que um erro de mapeamento ali não passe por aqui sem ser notado.
const SUGESTOES_QUE_PROPOEM_OPERACAO: readonly Caso[] = [
  {
    tela: 'skills',
    texto: 'Crie uma skill para reservar a quadra',
    permissao: 'skills.criar',
    alcance: 'catalogo',
  },
  {
    tela: 'skills',
    texto: 'Escreva o resumo das skills que estão sem',
    permissao: 'skills.editar_textos',
    alcance: 'catalogo',
  },
  {
    tela: 'skill',
    texto: 'Melhore o resumo desta skill',
    permissao: 'skills.editar_textos',
    alcance: 'catalogo',
  },
  {
    tela: 'skill',
    texto: 'Sugira frases de exemplo',
    permissao: 'skills.editar_textos',
    alcance: 'catalogo',
  },
  {
    tela: 'fluxo',
    texto: 'Adicione um Confirmar antes de gravar',
    permissao: 'skills.editar_fluxo',
    alcance: 'catalogo',
  },
  {
    tela: 'fluxo',
    texto: 'Pergunte o horário antes de reservar',
    permissao: 'skills.editar_fluxo',
    alcance: 'catalogo',
  },
  {
    tela: 'condominios',
    texto: 'Desligue a reserva de área comum onde não há área',
    permissao: 'condominios.habilitar',
    alcance: 'condominio',
  },
  {
    tela: 'condominio',
    texto: 'Escreva uma orientação sobre mudanças aos domingos',
    permissao: 'condominios.habilitar',
    alcance: 'condominio',
  },
  {
    tela: 'condominio',
    texto: 'Ajuste a antecedência da reserva para 3 dias',
    permissao: 'condominios.habilitar',
    alcance: 'condominio',
  },
  {
    tela: 'concierge',
    texto: 'Deixe a persona mais acolhedora',
    permissao: 'skills.editar_textos',
    alcance: 'catalogo',
  },
  {
    tela: 'concierge',
    texto: 'Revise a persona para respostas curtas',
    permissao: 'skills.editar_textos',
    alcance: 'catalogo',
  },
  {
    tela: 'avancado',
    texto: 'Revise este arquivo e proponha melhorias',
    permissao: 'studio.avancado',
    alcance: 'catalogo',
  },
];

describe('gating mirrors internal/copiloto/operacoes.go', () => {
  it.each(SUGESTOES_QUE_PROPOEM_OPERACAO)(
    'showsOnlyWhenServerAccepts($tela: $texto)',
    ({ tela, texto, permissao, alcance }) => {
      const comPermissao: Eu = {
        ...construtor,
        permissoes: [permissao],
        administradoraId: alcance === 'catalogo' ? null : 'adm-1',
      };
      const semPermissao: Eu = { ...comPermissao, permissoes: [] };

      expect(sugestoesPara({ tela }, comPermissao)).toContain(texto);
      expect(sugestoesPara({ tela }, semPermissao)).not.toContain(texto);
    },
  );

  // ferramentas e as perguntas (sem operação) das demais telas nunca aparecem aqui: não propõem
  // nada que o servidor valide, então não há regra de permissão para espelhar.
});
