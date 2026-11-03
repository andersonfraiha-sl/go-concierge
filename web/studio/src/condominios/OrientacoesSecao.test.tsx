import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { responderCom } from '@/test/fetchFalso';
import { gestor, rotasBase } from './estadoFalso';
import { abrirCondominio, gravador } from './paginaDeTeste';

describe('OrientacoesSecao', () => {
  it('savesGuidanceOneSecondAfterTyping', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/orientacao');
    responderCom({ ...rotasBase(gestor), ...handlers });
    const { user } = abrirCondominio();

    await user.type(
      await screen.findByRole('textbox', { name: 'Para todas as skills' }),
      'Fale com o zelador',
    );
    expect(corpos).toHaveLength(0);

    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos[0]).toEqual({
      condominio: 'vila-verde',
      geral: 'Fale com o zelador',
      por_caso: {},
    });
  });
});
