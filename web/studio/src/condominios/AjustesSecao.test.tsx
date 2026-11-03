import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { estadoFalso, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio, gravador } from './paginaDeTeste';

describe('AjustesSecao', () => {
  it('fillsAdjustmentsFromSuggestion', async () => {
    const ajustes = gravador('PUT /api/admin/ajustes');
    responderCom({
      ...rotasBase(gestor),
      ...ajustes.handlers,
      'POST /api/admin/sugerir-payload': () =>
        json({
          payload: { antecedencia_dias: 5, turno: 'tarde' },
          nota: 'Não entendi o protocolo.',
        }),
    });
    const { user } = abrirCondominio();

    // Duas ferramentas têm "Descreva em uma frase": a da skill (1a) e a geral "horario-portaria".
    const [campoDaSkill] = await screen.findAllByRole('textbox', { name: 'Descreva em uma frase' });
    if (!campoDaSkill) throw new Error('campo de sugestão não encontrado');
    await user.type(campoDaSkill, 'cinco dias, só à tarde');
    const [botaoDaSkill] = screen.getAllByRole('button', { name: 'Sugerir' });
    if (!botaoDaSkill) throw new Error('botão de sugestão não encontrado');
    await user.click(botaoDaSkill);

    await waitFor(() => expect(screen.getByLabelText('Antecedência mínima (dias)')).toHaveValue(5));
    expect(screen.getByText('Não entendi o protocolo.')).toBeInTheDocument();
    expect(ajustes.corpos).toHaveLength(0);

    await waitFor(() => expect(ajustes.corpos).toHaveLength(1), { timeout: 3000 });
    expect(ajustes.corpos[0]).toMatchObject({
      condominio: 'vila-verde',
      ajustes: { 'agenda-mudanca': { antecedencia_dias: 5, turno: 'tarde' } },
    });
  });

  it('clearsAdjustmentOverrideWithUsarOPadrao', async () => {
    const ajustes = gravador('PUT /api/admin/ajustes');
    const comValor = structuredClone(estadoFalso);
    comValor.mapeamento.payloads = {
      'vila-verde': { 'agenda-mudanca': { antecedencia_dias: 5 } },
    } as never;
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comValor),
      ...ajustes.handlers,
    });
    const { user } = abrirCondominio();

    await user.click(
      await screen.findByRole('button', { name: 'Usar o padrão: Antecedência mínima (dias)' }),
    );

    expect(screen.getByLabelText('Antecedência mínima (dias)')).toHaveValue(null);
    await waitFor(() => expect(ajustes.corpos).toHaveLength(1), { timeout: 3000 });
    expect(ajustes.corpos[0]).toMatchObject({
      ajustes: { 'agenda-mudanca': { antecedencia_dias: '' } },
    });
  });
});
