import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { responderCom } from '@/test/fetchFalso';
import { construtor, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio, gravador } from './paginaDeTeste';

describe('IdentificacaoSecao', () => {
  it('gestorCannotEditIdentification', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio();

    await screen.findByText('Skills neste condomínio');
    expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remover condomínio' })).not.toBeInTheDocument();
  });

  it('savesIdentificationOnExplicitClick', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/condominio');
    responderCom({ ...rotasBase(construtor), ...handlers });
    const { user } = abrirCondominio();

    const nome = await screen.findByRole('textbox', { name: 'Nome' });
    await user.clear(nome);
    await user.type(nome, 'Vila Verde II');
    expect(corpos).toHaveLength(0);
    await user.click(screen.getByRole('button', { name: 'Salvar' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({
        id: 'vila-verde',
        nome: 'Vila Verde II',
        administradora_id: 'adm-exemplo',
        account: 'acc-1',
        company_id: 'gruvi-1',
        tenant: 'atende-1',
      }),
    );
  });
});
