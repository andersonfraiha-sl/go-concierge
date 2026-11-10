import { describe, expect, it } from 'vitest';
import type { Diferenca } from './contratos';
import {
  aceitarTudo,
  type Escolhas,
  escolhasIniciais,
  paraEnvio,
  pendentes,
  recusarTudo,
} from './decisoes';

const dif = (trecho: string, extra: Partial<Diferenca> = {}): Diferenca => ({
  trecho,
  tipo: 'texto',
  rotulo: trecho,
  situacao: 'so_base',
  obrigatorio: false,
  antes: 'a',
  depois: 'b',
  minha: 'a',
  ...extra,
});

const soBase = dif('texto:resumo');
const conflito = dif('texto:titulo', { situacao: 'conflito', minha: 'c' });
const obrigatorio = dif('etapa:e-1', { tipo: 'etapa', obrigatorio: true });
const conflitoObrigatorio = dif('etapa:e-2', {
  tipo: 'etapa',
  situacao: 'conflito',
  obrigatorio: true,
});

describe('decisoes', () => {
  it('acceptsBaseOnlyByDefault', () => {
    expect(escolhasIniciais([soBase, conflito])).toEqual({
      'texto:resumo': { escolha: 'aceitar' },
    });
  });

  it('acceptsMandatoryEvenWhenConflicting', () => {
    expect(escolhasIniciais([obrigatorio, conflitoObrigatorio])).toEqual({
      'etapa:e-1': { escolha: 'aceitar' },
      'etapa:e-2': { escolha: 'aceitar' },
    });
  });

  it('leavesConflictsUndecided', () => {
    const iniciais = escolhasIniciais([conflito]);
    expect(pendentes([conflito], iniciais)).toEqual(['texto:titulo']);
  });

  it('rejectsAllWithKeepExceptMandatory', () => {
    expect(recusarTudo([soBase, conflito, obrigatorio])).toEqual({
      'texto:resumo': { escolha: 'manter' },
      'texto:titulo': { escolha: 'manter' },
      'etapa:e-1': { escolha: 'aceitar' },
    });
  });

  it('acceptsAllIncludingConflicts', () => {
    const todas = aceitarTudo([soBase, conflito]);
    expect(pendentes([soBase, conflito], todas)).toEqual([]);
    expect(Object.values(todas).every((e) => e.escolha === 'aceitar')).toBe(true);
  });

  it('countsPendingConflicts', () => {
    const outro = dif('texto:resumo2', { situacao: 'conflito' });
    const escolhas: Escolhas = { 'texto:titulo': { escolha: 'manter' } };
    expect(pendentes([soBase, conflito, outro], escolhas)).toEqual(['texto:resumo2']);
  });

  it('sendsMixWithItsText', () => {
    const escolhas: Escolhas = {
      'texto:titulo': { escolha: 'misturar', mistura: 'meio a meio' },
      'texto:resumo': { escolha: 'aceitar' },
    };
    expect(paraEnvio(escolhas)).toEqual([
      { trecho: 'texto:titulo', escolha: 'misturar', mistura: 'meio a meio' },
      { trecho: 'texto:resumo', escolha: 'aceitar' },
    ]);
  });
});
