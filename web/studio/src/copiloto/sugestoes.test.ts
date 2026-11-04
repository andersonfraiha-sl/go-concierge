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
