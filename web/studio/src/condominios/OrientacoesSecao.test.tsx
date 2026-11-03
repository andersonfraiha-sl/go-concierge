import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { estadoFalso, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio, gravador } from './paginaDeTeste';

describe('OrientacoesSecao', () => {
  it('savesGuidanceOneSecondAfterTyping', async () => {
    const comPorCaso = structuredClone(estadoFalso);
    comPorCaso.mapeamento.orientacoes = {
      'vila-verde': { geral: '', por_caso: { 'agendar-mudanca': 'Zelador acompanha.' } },
    } as never;
    const { corpos, handlers } = gravador('PUT /api/admin/orientacao');
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comPorCaso),
      ...handlers,
    });
    const { user } = abrirCondominio();

    await user.type(
      await screen.findByRole('textbox', { name: 'Orientações gerais' }),
      'Fale com o zelador',
    );
    expect(corpos).toHaveLength(0);

    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos[0]).toEqual({
      condominio: 'vila-verde',
      geral: 'Fale com o zelador',
      por_caso: { 'agendar-mudanca': 'Zelador acompanha.' },
    });
  });

  it('showsExampleAndHelp', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio();

    const campo = await screen.findByRole('textbox', { name: 'Orientações gerais' });
    expect(campo).toHaveAttribute(
      'placeholder',
      expect.stringMatching(/^Ex\.: A piscina fecha às 22h\./),
    );
    expect(
      screen.getByText(
        'Escreva como você explicaria a um atendente novo: horários, nomes, regras locais, o que não fazer. O concierge lê isso em toda conversa deste condomínio.',
      ),
    ).toBeInTheDocument();
  });

  it('showsDefaultGuidanceReadOnly', async () => {
    const comPadrao = structuredClone(estadoFalso);
    comPadrao.mapeamento.orientacoes = {
      '*': { geral: 'Silêncio após as 22h.', por_caso: {} },
    } as never;
    responderCom({ ...rotasBase(gestor), 'GET /api/admin/estado': () => json(comPadrao) });
    abrirCondominio();

    expect(await screen.findByText('Também vale o do Padrão:')).toBeInTheDocument();
    expect(screen.getByText('Silêncio após as 22h.')).toBeInTheDocument();
  });

  it('hidesPerSkillGuidance', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio();

    await screen.findByRole('textbox', { name: 'Orientações gerais' });
    for (const caso of estadoFalso.casos) {
      expect(screen.queryByRole('textbox', { name: caso.titulo })).not.toBeInTheDocument();
    }
  });
});
