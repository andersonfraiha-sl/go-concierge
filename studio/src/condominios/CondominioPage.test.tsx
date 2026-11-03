import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CondominioPage } from './CondominioPage';
import { construtor, estadoFalso, gestor, respostaMapeamento, rotasBase } from './estadoFalso';

type Corpo = Record<string, unknown>;

function gravador(rota: string) {
  const corpos: Corpo[] = [];
  const handlers = {
    [rota]: async (req: Request) => {
      corpos.push((await req.json()) as Corpo);
      return json(respostaMapeamento);
    },
  };
  return { corpos, handlers };
}

const abrir = (id = 'vila-verde') =>
  renderComProvedores(<CondominioPage />, { rota: `/condominios/${id}`, caminho: '/condominios/:id' });

describe('CondominioPage', () => {
  it('togglesSkill', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    responderCom({ ...rotasBase(gestor), ...handlers });
    const { user } = abrir();

    await user.click(await screen.findByRole('switch', { name: 'Agendar mudança neste condomínio' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({ condominio: 'vila-verde', caso: 'agendar-mudanca', habilitado: false }),
    );
  });

  it('resetsToDefault', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    const comDecisao = structuredClone(estadoFalso);
    comDecisao.mapeamento.habilitacoes['vila-verde'] = {
      'agendar-mudanca': { habilitado: false, papeis: [] },
    } as never;
    responderCom({ ...rotasBase(gestor), 'GET /api/admin/estado': () => json(comDecisao), ...handlers });
    const { user } = abrir();

    await user.click(await screen.findByRole('button', { name: 'Usar o padrão: Agendar mudança' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({ condominio: 'vila-verde', caso: 'agendar-mudanca', limpar: true }),
    );
  });

  it('savesRolesWhenChipIsToggled', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    responderCom({ ...rotasBase(gestor), ...handlers });
    const { user } = abrir();

    await user.click(await screen.findByRole('button', { name: 'síndico em Agendar mudança' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({ caso: 'agendar-mudanca', habilitado: true, papeis: ['sindico'] }),
    );
  });

  it('savesGuidanceOneSecondAfterTyping', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/orientacao');
    responderCom({ ...rotasBase(gestor), ...handlers });
    const { user } = abrir();

    await user.type(await screen.findByRole('textbox', { name: 'Para todas as skills' }), 'Fale com o zelador');
    expect(corpos).toHaveLength(0);

    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos[0]).toEqual({ condominio: 'vila-verde', geral: 'Fale com o zelador', por_caso: {} });
  });

  it('fillsAdjustmentsFromSuggestion', async () => {
    const ajustes = gravador('PUT /api/admin/ajustes');
    responderCom({
      ...rotasBase(gestor),
      ...ajustes.handlers,
      'POST /api/admin/sugerir-payload': () =>
        json({ payload: { antecedencia_dias: 5, turno: 'tarde' }, nota: 'Não entendi o protocolo.' }),
    });
    const { user } = abrir();

    await user.type(await screen.findByRole('textbox', { name: 'Descreva em uma frase' }), 'cinco dias, só à tarde');
    await user.click(screen.getByRole('button', { name: 'Sugerir' }));

    await waitFor(() => expect(screen.getByLabelText('Antecedência mínima (dias)')).toHaveValue(5));
    expect(screen.getByText('Não entendi o protocolo.')).toBeInTheDocument();
    expect(ajustes.corpos).toHaveLength(0);

    await waitFor(() => expect(ajustes.corpos).toHaveLength(1), { timeout: 3000 });
    expect(ajustes.corpos[0]).toMatchObject({
      condominio: 'vila-verde',
      ajustes: { 'agenda-mudanca': { antecedencia_dias: 5, turno: 'tarde' } },
    });
  });

  it('gestorCannotEditIdentification', async () => {
    responderCom(rotasBase(gestor));
    abrir();

    await screen.findByText('Skills neste condomínio');
    expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remover condomínio' })).not.toBeInTheDocument();
  });

  it('savesIdentificationOnExplicitClick', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/condominio');
    responderCom({ ...rotasBase(construtor), ...handlers });
    const { user } = abrir();

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

  it('showsDefaultWithoutIdentification', async () => {
    responderCom(rotasBase(construtor));
    abrir('*');

    expect(await screen.findByRole('heading', { level: 1, name: 'Padrão (todos)' })).toBeInTheDocument();
    expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
  });
});
