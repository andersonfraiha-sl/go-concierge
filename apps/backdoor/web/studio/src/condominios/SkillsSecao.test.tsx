import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { estadoFalso, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio, gravador } from './paginaDeTeste';

const estadoBase = structuredClone(estadoFalso);
const condoBase = estadoBase.mapeamento.condominios[0];
if (!condoBase) throw new Error('fixture sem condomínio');
condoBase.id = '101';
const rotas = () => ({ ...rotasBase(gestor), 'GET /api/admin/estado': () => json(estadoBase) });

describe('conteúdo por skill', () => {
  it('abreOrientacoesEAjustesSemControlesDeAtivacao', async () => {
    responderCom(rotas());
    const { user } = abrirCondominio('101');

    await user.click(await screen.findByRole('button', { name: 'Agendar mudança' }));

    expect(screen.getByText('Orientação desta skill')).toBeInTheDocument();
    expect(screen.getByText('Ajustes desta skill')).toBeInTheDocument();
    expect(
      screen.queryByRole('switch', { name: /neste condomínio|Ligada/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/ligadas|desligada aqui|só síndico/)).not.toBeInTheDocument();
  });

  it('salvaOrientacaoDaSkillPreservandoAGeralEOutrasSkills', async () => {
    const estado = structuredClone(estadoBase);
    estado.mapeamento.orientacoes = {
      '101': { geral: 'Piscina às 22h.', por_caso: { reserva_salao: 'Sem som após 22h.' } },
    };
    const { corpos, handlers } = gravador('PUT /api/admin/orientacao');
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(estado),
      ...handlers,
    });
    const { user } = abrirCondominio('101');

    await user.click(await screen.findByRole('button', { name: 'Agendar mudança' }));
    await user.type(screen.getByLabelText('Orientação de Agendar mudança'), 'Chamar o zelador.');

    await waitFor(
      () =>
        expect(corpos[0]).toMatchObject({
          company_id: 'adm-exemplo',
          condo_id: 101,
          geral: 'Piscina às 22h.',
          por_caso: {
            agendar_mudanca: 'Chamar o zelador.',
            reserva_salao: 'Sem som após 22h.',
          },
        }),
      { timeout: 3000 },
    );
  });

  it('salvaAjustesDaSkillPreservandoOutrasFerramentas', async () => {
    const estado = structuredClone(estadoBase);
    estado.mapeamento.payloads = { '101': { 'horario-portaria': { horario: '8h às 20h' } } };
    const { corpos, handlers } = gravador('PUT /api/admin/ajustes');
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(estado),
      ...handlers,
    });
    const { user } = abrirCondominio('101');

    await user.click(await screen.findByRole('button', { name: 'Agendar mudança' }));
    const campo = screen.getByLabelText('Antecedência mínima');
    await user.clear(campo);
    await user.type(campo, '5');

    await waitFor(
      () =>
        expect(corpos[0]).toMatchObject({
          ajustes: {
            'horario-portaria': { horario: '8h às 20h' },
            'agenda-mudanca': { antecedencia_dias: 5 },
          },
        }),
      { timeout: 3000 },
    );
  });
});
