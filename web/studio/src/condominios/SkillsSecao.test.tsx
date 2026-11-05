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

  it('showsCountOfEnabled', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio();

    expect(await screen.findByText('1 de 2 ligadas')).toBeInTheDocument();
  });

  it('filtersBySearch', async () => {
    responderCom(rotasBase(gestor));
    const { user } = abrirCondominio();
    await screen.findByText('Agendar mudança');

    await user.type(screen.getByLabelText('Buscar skill'), 'reserva');

    expect(screen.getByText('Reserva do salão')).toBeInTheDocument();
    expect(screen.queryByText('Agendar mudança')).not.toBeInTheDocument();
  });

  it('showsOnlyChangesFromDefault', async () => {
    const comUmaDecisaoPropria = structuredClone(estadoFalso);
    comUmaDecisaoPropria.mapeamento.habilitacoes['vila-verde'] = {
      'agendar-mudanca': { habilitado: false, papeis: [] },
    } as never;
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comUmaDecisaoPropria),
    });
    const { user } = abrirCondominio();
    await screen.findByText('Agendar mudança');

    await user.click(screen.getByLabelText('Só o que muda do Padrão'));

    expect(screen.getByText('Agendar mudança')).toBeInTheDocument();
    expect(screen.queryByText('Reserva do salão')).not.toBeInTheDocument();
  });

  it('groupsByCategory', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio();

    expect(await screen.findByText('Registram no sistema')).toBeInTheDocument();
  });

  it('showsRoleChipOnlyWhenRestricted', async () => {
    const comPapelRestrito = structuredClone(estadoFalso);
    comPapelRestrito.mapeamento.habilitacoes['vila-verde'] = {
      'agendar-mudanca': { habilitado: true, papeis: ['sindico'] },
    } as never;
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comPapelRestrito),
    });
    abrirCondominio();

    expect(await screen.findByText('só síndico')).toBeInTheDocument();
  });

  it('showsOwnDecisionChip', async () => {
    const comDecisaoPropria = structuredClone(estadoFalso);
    comDecisaoPropria.mapeamento.habilitacoes['vila-verde'] = {
      'reserva-do-salao': { habilitado: false, papeis: [] },
    } as never;
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comDecisaoPropria),
    });
    abrirCondominio();

    expect(await screen.findByText('desligada aqui')).toBeInTheDocument();
  });

  it('opensDrawerOnRowClick', async () => {
    responderCom(rotasBase(gestor));
    const { user } = abrirCondominio();

    await user.click(await screen.findByText('Agendar mudança'));

    expect(await screen.findByRole('dialog', { name: /Agendar mudança/ })).toBeInTheDocument();
  });

  it('toggleDoesNotOpenDrawer', async () => {
    const { handlers } = gravador('PUT /api/admin/habilitacao');
    responderCom({ ...rotasBase(gestor), ...handlers });
    const { user } = abrirCondominio();

    await user.click(
      await screen.findByRole('switch', { name: 'Agendar mudança neste condomínio' }),
    );

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('hidesOnlyChangesToggleOnDefault', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio('*');

    await screen.findByText('Agendar mudança');
    expect(screen.queryByLabelText('Só o que muda do Padrão')).not.toBeInTheDocument();
  });

  it('showsNoResultsMessage', async () => {
    responderCom(rotasBase(gestor));
    const { user } = abrirCondominio();

    await user.type(await screen.findByLabelText('Buscar skill'), 'inexistente');

    expect(await screen.findByText('Nenhuma skill encontrada.')).toBeInTheDocument();
  });
});
