import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { estadoFalso, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio, gravador } from './paginaDeTeste';

describe('AjustesSecao', () => {
  it('showsOnlyToolsOutsideSkills', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio();

    expect(await screen.findByLabelText('Horário da portaria')).toBeInTheDocument();
    expect(screen.queryByLabelText('Antecedência mínima (dias)')).not.toBeInTheDocument();
  });

  it('keepsSkillAdjustmentsWhenSaving', async () => {
    const ajustes = gravador('PUT /api/admin/ajustes');
    const comPayloadDaSkill = structuredClone(estadoFalso);
    comPayloadDaSkill.mapeamento.payloads = {
      'vila-verde': { 'agenda-mudanca': { antecedencia_dias: 5, turno: 'tarde' } },
    } as never;
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comPayloadDaSkill),
      ...ajustes.handlers,
    });
    const { user } = abrirCondominio();

    const campo = await screen.findByLabelText('Horário da portaria');
    await user.type(campo, '8h às 20h');

    await waitFor(() => expect(ajustes.corpos).toHaveLength(1), { timeout: 3000 });
    expect(ajustes.corpos[0]).toMatchObject({
      condominio: 'vila-verde',
      ajustes: {
        'agenda-mudanca': { antecedencia_dias: 5, turno: 'tarde' },
        'horario-portaria': { horario: '8h às 20h' },
      },
    });
  });

  it('fillsAdjustmentsFromSuggestion', async () => {
    const ajustes = gravador('PUT /api/admin/ajustes');
    responderCom({
      ...rotasBase(gestor),
      ...ajustes.handlers,
      'POST /api/admin/sugerir-payload': () =>
        json({ payload: { horario: '8h às 20h' }, nota: 'Ajustei o horário.' }),
    });
    const { user } = abrirCondominio();

    await user.click(await screen.findByRole('button', { name: 'Preencher com IA' }));
    const campoDaFrase = await screen.findByRole('textbox', { name: 'Descreva em uma frase' });
    await user.type(campoDaFrase, 'abre às oito e fecha às oito da noite');
    await user.click(screen.getByRole('button', { name: 'Sugerir' }));

    await waitFor(() =>
      expect(screen.getByLabelText('Horário da portaria')).toHaveValue('8h às 20h'),
    );
    expect(screen.getByText('Ajustei o horário.')).toBeInTheDocument();

    await waitFor(() => expect(ajustes.corpos).toHaveLength(1), { timeout: 3000 });
    expect(ajustes.corpos[0]).toMatchObject({
      condominio: 'vila-verde',
      ajustes: { 'horario-portaria': { horario: '8h às 20h' } },
    });
  });

  it('clearsAdjustmentOverrideWithUsarOPadrao', async () => {
    const ajustes = gravador('PUT /api/admin/ajustes');
    const comValor = structuredClone(estadoFalso);
    comValor.mapeamento.payloads = {
      'vila-verde': { 'horario-portaria': { horario: '6h às 23h' } },
    } as never;
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comValor),
      ...ajustes.handlers,
    });
    const { user } = abrirCondominio();

    await user.click(
      await screen.findByRole('button', { name: 'Usar o padrão: Horário da portaria' }),
    );

    expect(screen.getByLabelText('Horário da portaria')).toHaveValue('');
    await waitFor(() => expect(ajustes.corpos).toHaveLength(1), { timeout: 3000 });
    expect(ajustes.corpos[0]).toMatchObject({
      ajustes: { 'horario-portaria': { horario: '' } },
    });
  });
});
