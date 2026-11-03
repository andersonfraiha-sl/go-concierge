import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { estadoFalso, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio, gravador } from './paginaDeTeste';

describe('SkillsSecao', () => {
  it('togglesSkill', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    responderCom({ ...rotasBase(gestor), ...handlers });
    const { user } = abrirCondominio();

    await user.click(
      await screen.findByRole('switch', { name: 'Agendar mudança neste condomínio' }),
    );

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({
        condominio: 'vila-verde',
        caso: 'agendar-mudanca',
        habilitado: false,
      }),
    );
  });

  it('resetsToDefault', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    const comDecisao = structuredClone(estadoFalso);
    comDecisao.mapeamento.habilitacoes['vila-verde'] = {
      'agendar-mudanca': { habilitado: false, papeis: [] },
    } as never;
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comDecisao),
      ...handlers,
    });
    const { user } = abrirCondominio();

    await user.click(await screen.findByRole('button', { name: 'Usar o padrão: Agendar mudança' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({
        condominio: 'vila-verde',
        caso: 'agendar-mudanca',
        limpar: true,
      }),
    );
  });

  it('savesRolesWhenChipIsToggled', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    responderCom({ ...rotasBase(gestor), ...handlers });
    const { user } = abrirCondominio();

    await user.click(await screen.findByRole('button', { name: 'síndico em Agendar mudança' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({
        caso: 'agendar-mudanca',
        habilitado: true,
        papeis: ['sindico'],
      }),
    );
  });
});
