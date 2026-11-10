import { describe, expect, it } from 'vitest';
import type { Bloco } from '@/fluxo/modelo/tipos';
import { novoId } from './fluxoDaSkill';

const PADRAO_DO_ID = /^e-[0-9a-f]{8}$/;

describe('novoId', () => {
  it('createsRandomIdsThatNeverLookPositional', () => {
    const ids = Array.from({ length: 50 }, () => novoId([]));
    expect(new Set(ids).size).toBe(50);
    for (const id of ids) expect(id).toMatch(PADRAO_DO_ID);
  });

  it('skipsIdsAlreadyUsedByTheFlow', () => {
    const id = novoId([]);
    const existente: Bloco = { id, tipo: 'confirmar', texto: '' };
    expect(novoId([existente])).not.toBe(id);
  });
});
