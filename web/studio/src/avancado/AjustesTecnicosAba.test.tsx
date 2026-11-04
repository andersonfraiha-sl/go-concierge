import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { construtor, estadoFalso, rotasBase } from '@/condominios/estadoFalso';
import { gravador } from '@/condominios/paginaDeTeste';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { AjustesTecnicosAba } from './AjustesTecnicosAba';

const estado = structuredClone(estadoFalso);
estado.mapeamento.payloads = {
  '*': { 'reserva-salao': { limite: 1 }, 'agenda-mudanca': { turno: 'manhã' } },
  'vila-verde': {
    'agenda-mudanca': { turno: 'tarde', faixas: [1, 2], nulo: null, ativo: true },
  },
};

const rotas = () => ({ ...rotasBase(construtor), 'GET /api/admin/estado': () => json(estado) });

async function escolherVilaVerde(user: ReturnType<typeof renderComProvedores>['user']) {
  await user.click(await screen.findByRole('combobox', { name: 'Condomínio' }));
  await user.click(await screen.findByRole('option', { name: 'Vila Verde' }));
}

const abrirReserva = async (user: ReturnType<typeof renderComProvedores>['user']) => {
  await escolherVilaVerde(user);
  await user.click(screen.getByRole('button', { name: /Reserva do salão/ }));
  await user.click(await screen.findByRole('button', { name: 'Adicionar valor' }));
};

describe('AjustesTecnicosAba', () => {
  it('marksOwnAndInherited', async () => {
    responderCom(rotas());
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await escolherVilaVerde(user);

    const propria = screen.getByRole('button', { name: /Agenda de mudança/ });
    expect(within(propria).getByText('próprio')).toBeInTheDocument();
    const herdada = screen.getByRole('button', { name: /Reserva do salão/ });
    expect(within(herdada).getByText('herdado')).toBeInTheDocument();
    const nenhuma = screen.getByRole('button', { name: /Horário da portaria/ });
    expect(within(nenhuma).queryByText(/próprio|herdado/)).toBeNull();
  });

  it('savesFreePayload', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/payload');
    responderCom({ ...rotas(), ...handlers });
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await escolherVilaVerde(user);

    await user.click(screen.getByRole('button', { name: /Reserva do salão/ }));
    await user.click(await screen.findByRole('button', { name: 'Adicionar valor' }));
    await user.type(screen.getByRole('textbox', { name: 'Chave' }), 'limite');
    await user.type(screen.getByRole('textbox', { name: 'Valor' }), '3');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));

    await waitFor(() =>
      expect(corpos[0]).toEqual({
        condominio: 'vila-verde',
        recurso: 'reserva-salao',
        payload: { limite: 3 },
        apagar: false,
      }),
    );
  });

  it('clearsPayloadForCondominio', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/payload');
    responderCom({ ...rotas(), ...handlers });
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await escolherVilaVerde(user);

    await user.click(screen.getByRole('button', { name: /Agenda de mudança/ }));
    await user.click(await screen.findByRole('button', { name: 'Limpar deste condomínio' }));
    await user.click(
      within(await screen.findByRole('dialog', { name: /Limpar/ })).getByRole('button', {
        name: 'Limpar',
      }),
    );

    await waitFor(() =>
      expect(corpos[0]).toEqual({
        condominio: 'vila-verde',
        recurso: 'agenda-mudanca',
        payload: {},
        apagar: true,
      }),
    );
  });

  it('keepsZipCodeAsTextWhenTypeIsChosen', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/payload');
    responderCom({ ...rotas(), ...handlers });
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await abrirReserva(user);
    await user.type(screen.getByRole('textbox', { name: 'Chave' }), 'cep');
    await user.type(screen.getByRole('textbox', { name: 'Valor' }), '01310');
    await user.selectOptions(screen.getByRole('combobox', { name: 'Tipo do valor' }), 'texto');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));

    await waitFor(() => expect(corpos[0]?.payload).toEqual({ cep: '01310' }));
  });

  it('savesUntouchedArrayAndNullUnchanged', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/payload');
    responderCom({ ...rotas(), ...handlers });
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await escolherVilaVerde(user);
    await user.click(screen.getByRole('button', { name: /Agenda de mudança/ }));
    await user.click(await screen.findByRole('button', { name: 'Salvar' }));

    await waitFor(() =>
      expect(corpos[0]?.payload).toEqual({
        turno: 'tarde',
        faixas: [1, 2],
        nulo: null,
        ativo: true,
      }),
    );
  });

  it('showsFieldErrorsForInvalidJsonAndDuplicateKeys', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/payload');
    responderCom({ ...rotas(), ...handlers });
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await abrirReserva(user);
    await user.type(screen.getByRole('textbox', { name: 'Chave' }), 'a');
    await user.selectOptions(screen.getByRole('combobox', { name: 'Tipo do valor' }), 'JSON');
    await user.type(screen.getByRole('textbox', { name: 'Valor' }), '{{');
    await user.click(screen.getByRole('button', { name: 'Adicionar valor' }));
    await user.type(screen.getAllByRole('textbox', { name: 'Chave' })[1] as HTMLElement, 'a');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));

    expect(await screen.findByText('JSON inválido')).toBeInTheDocument();
    expect(screen.getByText('Chave repetida')).toBeInTheDocument();
    expect(corpos).toHaveLength(0);
  });

  it('keepsDrawerOpenAndReportsServerError', async () => {
    responderCom({
      ...rotas(),
      'PUT /api/admin/payload': () => json({ mensagem: 'Servidor recusou' }, 500),
    });
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await abrirReserva(user);
    await user.type(screen.getByRole('textbox', { name: 'Chave' }), 'limite');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));

    expect(await screen.findByText('Servidor recusou')).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Chave' })).toHaveValue('limite');
  });

  it('disablesEditingWhenReadOnly', async () => {
    responderCom(rotas());
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={false} />);
    await escolherVilaVerde(user);
    await user.click(screen.getByRole('button', { name: /Agenda de mudança/ }));

    expect(await screen.findByRole('button', { name: 'Salvar' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Limpar deste condomínio' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Adicionar valor' })).toBeDisabled();
  });

  it('keepsPayloadWhenClearIsCancelled', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/payload');
    responderCom({ ...rotas(), ...handlers });
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await escolherVilaVerde(user);
    await user.click(screen.getByRole('button', { name: /Agenda de mudança/ }));
    await user.click(await screen.findByRole('button', { name: 'Limpar deste condomínio' }));
    await user.click(
      within(await screen.findByRole('dialog', { name: /Limpar/ })).getByRole('button', {
        name: 'Cancelar',
      }),
    );

    expect(corpos).toHaveLength(0);
  });

  it('disablesClearWhenNothingIsOwn', async () => {
    responderCom(rotas());
    const { user } = renderComProvedores(<AjustesTecnicosAba editar={true} />);
    await escolherVilaVerde(user);
    await user.click(screen.getByRole('button', { name: /Reserva do salão/ }));

    expect(await screen.findByRole('button', { name: 'Limpar deste condomínio' })).toBeDisabled();
  });
});
