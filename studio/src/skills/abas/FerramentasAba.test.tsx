import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { FerramentasAba } from './FerramentasAba';
import { AGENTE, agenteLido, construtor, ferramenta, gestor, recursoDeclarado } from './fixturesDeAbas';

const catalogo = {
  account: 'superlogica',
  origem: 'instantaneo',
  conferido: true,
  recursos: [
    ferramenta('unidades.consultar', false, 'Consultar unidade'),
    ferramenta('boletos.listar', false, 'Listar boletos'),
    ferramenta('mudancas.agendar', true, 'Agendar mudança'),
  ],
};

const rotas = (eu: Eu, extras: Parameters<typeof responderCom>[0] = {}) => ({
  'GET /api/eu': () => json(eu),
  'GET /api/agente': () =>
    json(
      agenteLido({
        recursos: [
          recursoDeclarado('mudancas.agendar', 'caso.yaml'),
          recursoDeclarado('unidades.consultar', 'agent.yaml'),
        ],
      }),
    ),
  'GET /api/nexus/recursos': () => json(catalogo),
  ...extras,
});

const montar = () => renderComProvedores(<FerramentasAba nome="agendar-mudanca" caminho={AGENTE} />);

describe('FerramentasAba', () => {
  it('flowToolsAreLocked', async () => {
    responderCom(rotas(construtor));
    montar();
    expect(await screen.findByText('Usada no fluxo')).toBeInTheDocument();
    expect(screen.queryByLabelText('Usar mudancas.agendar')).not.toBeInTheDocument();
    expect(screen.getByLabelText('Usar unidades.consultar')).toBeInTheDocument();
  });

  it('togglesDeclaredTool', async () => {
    const corpos: unknown[] = [];
    responderCom(
      rotas(construtor, {
        'POST /api/agente/recursos': async (req: Request) => {
          corpos.push(await req.json());
          return json({ ligado: false });
        },
      }),
    );
    const { user } = montar();
    const chave = await screen.findByLabelText('Usar unidades.consultar');
    await waitFor(() => expect(chave).toBeEnabled());
    await user.click(chave);
    await waitFor(() => expect(corpos).toEqual([{ agente: AGENTE, nome: 'unidades.consultar', ligar: false }]));
  });

  it('addsOnlyReadTools', async () => {
    responderCom(rotas(construtor));
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Adicionar ferramenta' }));
    const lista = await screen.findByRole('dialog');
    expect(await within(lista).findByText('Listar boletos')).toBeInTheDocument();
    expect(within(lista).queryByText('Agendar mudança')).not.toBeInTheDocument();
    expect(within(lista).queryByText('Consultar unidade')).not.toBeInTheDocument();
  });

  it('gestorReadsOnly', async () => {
    responderCom(rotas(gestor));
    montar();
    const chave = await screen.findByLabelText('Usar unidades.consultar');
    expect(chave).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Adicionar ferramenta' })).not.toBeInTheDocument();
  });
});
