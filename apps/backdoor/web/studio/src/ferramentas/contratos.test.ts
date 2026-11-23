import { describe, expect, it } from 'vitest';
import {
  type Cabecalho,
  cabecalhosEmLista,
  listaEmCabecalhos,
  padraoDoAjuste,
  paraGravar,
  validarCodigo,
} from './contratos';

describe('cabecalhos', () => {
  it('roundTripsHeaders', () => {
    const headers = { Authorization: 'Bearer x', 'X-Conta': '123' };
    const lista = cabecalhosEmLista(headers);
    expect(lista).toEqual([
      { chave: 'Authorization', valor: 'Bearer x' },
      { chave: 'X-Conta', valor: '123' },
    ]);
    expect(listaEmCabecalhos(lista)).toEqual(headers);
  });

  it('dropsEmptyKeysAndCollapsesEmptyListToUndefined', () => {
    const lista: Cabecalho[] = [{ chave: '', valor: 'sobra' }];
    expect(listaEmCabecalhos(lista)).toBeUndefined();
    expect(listaEmCabecalhos([])).toBeUndefined();
  });

  it('returnsEmptyListForUndefinedHeaders', () => {
    expect(cabecalhosEmLista(undefined)).toEqual([]);
  });
});

describe('padraoDoAjuste', () => {
  it.each([
    { tipo: 'string', texto: '', esperado: undefined },
    { tipo: 'string', texto: 'abc', esperado: 'abc' },
    { tipo: 'number', texto: '3', esperado: 3 },
    { tipo: 'number', texto: 'x', esperado: undefined },
    { tipo: 'boolean', texto: 'sim', esperado: true },
    { tipo: 'boolean', texto: 'não', esperado: false },
    { tipo: undefined, texto: 'texto livre', esperado: 'texto livre' },
  ])('parsesAdjustmentDefault($tipo, $texto)', ({ tipo, texto, esperado }) => {
    expect(padraoDoAjuste(tipo, texto)).toEqual(esperado);
  });
});

describe('validarCodigo', () => {
  it.each([
    { codigo: 'reservas.criar', valido: true },
    { codigo: 'grupo.acao.sub', valido: true },
    { codigo: 'reservas_antigas.criar-v2', valido: true },
    { codigo: 'reservas', valido: false },
    { codigo: 'Reservas.Criar', valido: false },
    { codigo: 'reservas.', valido: false },
    { codigo: '', valido: false },
  ])('validatesToolCode($codigo)', ({ codigo, valido }) => {
    expect(validarCodigo(codigo)).toBe(
      valido ? null : 'Use grupo.acao, em minúsculas, como reservas.criar.',
    );
  });
});

describe('paraGravar', () => {
  const amostra = { ok: true as const, valor: undefined };
  const base = {
    name: 'reservas.criar',
    kind: 'api',
    visibility: 'private',
    write: false,
    description: '',
    params: [],
    saida: [],
    definicoes: [],
  };

  it('dropsOptionsOutsideChoice', () => {
    const gravado = paraGravar(
      {
        ...base,
        definicoes: [
          { name: 'turno', label: 'Turno', type: 'select', options: ['manha', 'tarde'] },
          { name: 'dias', label: 'Dias', type: 'number', options: ['3', '5'] },
        ],
      },
      amostra,
    );
    expect(gravado.definicoes).toEqual([
      { name: 'turno', label: 'Turno', type: 'select', options: ['manha', 'tarde'] },
      { name: 'dias', label: 'Dias', type: 'number', options: [] },
    ]);
  });
});
