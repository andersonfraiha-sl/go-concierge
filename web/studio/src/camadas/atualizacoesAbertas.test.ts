import { describe, expect, it } from 'vitest';
import {
  atualizacoesDaCamada,
  avisosDaCamada,
  descreverTrechos,
  skillsComAtualizacao,
} from './atualizacoesAbertas';
import { AtualizacoesSchema, type Camada } from './contratos';

const adm1: Camada = { skill: 'mudanca', tipo: 'administradora', id: 'adm-1' };

const resumo = (camada: Camada, estado: string, id = 'at-1') => ({
  id,
  camada,
  estado,
  para: { numero: 2, resumo: '' },
  prazoEm: null,
  trechos: 3,
  conflitos: 1,
});

const dados = AtualizacoesSchema.parse({
  atualizacoes: [
    resumo(adm1, 'pendente'),
    resumo(adm1, 'recusada', 'at-2'),
    resumo({ ...adm1, id: 'adm-2' }, 'pendente', 'at-3'),
    resumo({ ...adm1, skill: 'faq' }, 'aceita', 'at-4'),
  ],
  avisos: [
    { id: 'av-1', camada: adm1, motivo: 'm', trechos: [], encerraEm: null, criadoEm: '2026-11-04' },
    {
      id: 'av-2',
      camada: { ...adm1, id: 'adm-2' },
      motivo: 'm',
      trechos: [],
      encerraEm: null,
      criadoEm: '2026-11-04',
    },
  ],
});

describe('atualizacoesAbertas', () => {
  it('keepsOnlyPendingUpdatesOfTheLayer', () => {
    expect(atualizacoesDaCamada(dados, adm1).map((a) => a.id)).toEqual(['at-1']);
  });

  it('keepsOnlyNoticesOfTheLayer', () => {
    expect(avisosDaCamada(dados, adm1).map((a) => a.id)).toEqual(['av-1']);
  });

  it('listsSkillsWithPendingUpdate', () => {
    expect([...skillsComAtualizacao(dados)]).toEqual(['mudanca']);
  });

  it.each([
    { trechos: ['texto:titulo'], esperado: 'o título' },
    { trechos: ['texto:titulo', 'orientacao'], esperado: 'o título e a orientação' },
    {
      trechos: ['texto:resumo', 'etapa:e-1', 'etapa:e-2', 'fluxo:ordem'],
      esperado: 'o resumo, a ordem das etapas e 2 etapas do passo a passo',
    },
    { trechos: ['etapa:e-1'], esperado: 'uma etapa do passo a passo' },
    { trechos: ['informacao:cpf'], esperado: 'a informação "cpf"' },
  ])('describes $trechos', ({ trechos, esperado }) => {
    expect(descreverTrechos(trechos)).toBe(esperado);
  });
});
