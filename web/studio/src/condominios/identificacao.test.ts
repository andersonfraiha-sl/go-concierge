import { describe, expect, it } from 'vitest';
import type { Condominio } from './contratos';
import { validarIdentificacao } from './identificacao';

const BASE: Condominio = {
  id: 'vila-verde',
  nome: 'Vila Verde',
  administradora_id: '',
  account: '',
  company_id: '',
  tenant: '',
};

describe('validarIdentificacao', () => {
  it.each([
    { parcial: {}, esperado: {} },
    { parcial: { nome: '' }, esperado: { nome: 'Informe o nome.' } },
    { parcial: { nome: '   ' }, esperado: { nome: 'Informe o nome.' } },
    { parcial: { account: 'cond-001' }, esperado: {} },
    {
      parcial: { account: 'Cond 001' },
      esperado: { account: 'Use letras minúsculas, números e hífen, como cond-001.' },
    },
    { parcial: { company_id: '5982' }, esperado: {} },
    { parcial: { company_id: 'abc' }, esperado: { company_id: 'Use só números, como 5982.' } },
    { parcial: { tenant: '69a5a131e67bd8e2f54e5981' }, esperado: {} },
    {
      parcial: { tenant: 'xyz' },
      esperado: { tenant: 'São 24 caracteres, de 0 a 9 e de a a f.' },
    },
  ])('validatesEachFormat $parcial', ({ parcial, esperado }) => {
    expect(validarIdentificacao({ ...BASE, ...parcial })).toEqual(esperado);
  });
});
