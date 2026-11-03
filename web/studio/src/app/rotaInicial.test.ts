import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { rotaInicial } from './rotaInicial';

const base: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: [],
  administradoraId: null,
  dev: false,
};

const casos: { eu: Eu; esperado: '/condominios' | '/skills' }[] = [
  {
    eu: {
      ...base,
      papel: 'gestor',
      administradoraId: 'adm-exemplo',
      permissoes: ['condominios.habilitar'],
    },
    esperado: '/condominios',
  },
  { eu: { ...base, papel: 'construtor', administradoraId: null }, esperado: '/skills' },
  {
    eu: { ...base, papel: 'gestor', administradoraId: 'adm-exemplo', permissoes: [] },
    esperado: '/skills',
  },
];

describe('rotaInicial', () => {
  it.each(casos)('picksStartPageByRole($eu.papel)', ({ eu, esperado }) => {
    expect(rotaInicial(eu)).toBe(esperado);
  });
});
