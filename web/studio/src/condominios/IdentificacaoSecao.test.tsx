import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { construtor, estadoFalso, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio, gravador } from './paginaDeTeste';

describe('IdentificacaoSecao', () => {
  it('gestorCannotEditIdentification', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio();

    await screen.findByText('O que o concierge faz aqui');
    expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remover condomínio' })).not.toBeInTheDocument();
  });

  it('savesIdentificationAutomatically', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/condominio');
    responderCom({ ...rotasBase(construtor), ...handlers });
    const { user } = abrirCondominio();

    const nome = await screen.findByRole('textbox', { name: 'Nome' });
    await user.clear(nome);
    await user.type(nome, 'Vila Verde II');
    expect(corpos).toHaveLength(0);

    await waitFor(
      () =>
        expect(corpos[0]).toMatchObject({
          id: 'vila-verde',
          nome: 'Vila Verde II',
          administradora_id: 'adm-exemplo',
          account: 'acc-1',
          company_id: '100',
          tenant: 'aaaaaaaaaaaaaaaaaaaaaaaa',
        }),
      { timeout: 3000 },
    );
  });

  it('keepsInvalidTenantUnsavedWhileSendingTheRest', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/condominio');
    responderCom({ ...rotasBase(construtor), ...handlers });
    const { user } = abrirCondominio();

    const tenant = await screen.findByRole('textbox', { name: 'Canal do Atende' });
    await user.clear(tenant);
    await user.type(tenant, 'xyz');

    await screen.findByText('São 24 caracteres, de 0 a 9 e de a a f.');

    await waitFor(() => expect(corpos[0]).toMatchObject({ tenant: 'aaaaaaaaaaaaaaaaaaaaaaaa' }), {
      timeout: 3000,
    });
    expect(screen.getByRole('textbox', { name: 'Canal do Atende' })).toHaveValue('xyz');
  });

  it('savesNewNomeWithLastValidTenantWhenTenantIsInvalid', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/condominio');
    responderCom({ ...rotasBase(construtor), ...handlers });
    const { user } = abrirCondominio();

    const tenant = await screen.findByRole('textbox', { name: 'Canal do Atende' });
    await user.clear(tenant);
    await user.type(tenant, 'xyz');
    const nome = screen.getByRole('textbox', { name: 'Nome' });
    await user.clear(nome);
    await user.type(nome, 'Vila Verde III');

    await waitFor(
      () =>
        expect(corpos[0]).toMatchObject({
          nome: 'Vila Verde III',
          tenant: 'aaaaaaaaaaaaaaaaaaaaaaaa',
        }),
      { timeout: 3000 },
    );
  });

  it('flushesValidEditOnUnmountDespiteInvalidField', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/condominio');
    responderCom({ ...rotasBase(construtor), ...handlers });
    const { user, unmount } = abrirCondominio();

    const tenant = await screen.findByRole('textbox', { name: 'Canal do Atende' });
    await user.clear(tenant);
    await user.type(tenant, 'xyz');
    const nome = screen.getByRole('textbox', { name: 'Nome' });
    await user.clear(nome);
    await user.type(nome, 'Vila Verde IV');

    unmount();

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({
        nome: 'Vila Verde IV',
        tenant: 'aaaaaaaaaaaaaaaaaaaaaaaa',
      }),
    );
  });

  it('showsIdAndPlaceholders', async () => {
    responderCom(rotasBase(construtor));
    abrirCondominio();

    const id = await screen.findByRole('textbox', { name: 'Id' });
    expect(id).toHaveValue('vila-verde');
    expect(id).toBeDisabled();
    expect(screen.getByRole('textbox', { name: 'Conta de integrações' })).toHaveAttribute(
      'placeholder',
      'cond-001',
    );
    expect(screen.getByRole('textbox', { name: 'Empresa no Gruvi' })).toHaveAttribute(
      'placeholder',
      '5982',
    );
    expect(screen.getByRole('textbox', { name: 'Canal do Atende' })).toHaveAttribute(
      'placeholder',
      '69a5a131e67bd8e2f54e5981',
    );
  });

  it('flagsMissingIds', async () => {
    const semIds = structuredClone(estadoFalso);
    const [vilaVerde] = semIds.mapeamento.condominios;
    if (vilaVerde === undefined) throw new Error('fixture sem condomínio vila-verde');
    vilaVerde.tenant = '';
    responderCom({ ...rotasBase(construtor), 'GET /api/admin/estado': () => json(semIds) });
    abrirCondominio();

    expect(await screen.findByText('Faltam ids de integração')).toBeInTheDocument();
  });
});
