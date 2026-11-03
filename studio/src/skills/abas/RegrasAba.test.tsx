import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { caminhoDeRegra } from '@/catalogo/contratos';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { AGENTE, agenteLido, arquivo, construtor } from './fixturesDeAbas';
import { RegrasAba } from './RegrasAba';

const politicas = caminhoDeRegra('politicas.md');

const montar = () => renderComProvedores(<RegrasAba nome="agendar-mudanca" caminho={AGENTE} />);

const rotasBase = () => ({
  'GET /api/eu': () => json(construtor),
  'GET /api/agente': () =>
    json(
      agenteLido({
        regras: [
          arquivo(politicas),
          arquivo(caminhoDeRegra('output.md')),
          arquivo(caminhoDeRegra('tom-de-voz.md')),
        ],
      }),
    ),
  'GET /api/arquivo': () => json({ caminho: politicas, conteudo: 'Seja gentil.' }),
});

describe('RegrasAba', () => {
  it('listsRulesWithReadableTitles', async () => {
    responderCom(rotasBase());
    montar();
    expect(await screen.findByRole('textbox', { name: 'Politicas' })).toHaveValue('Seja gentil.');
    expect(screen.getByRole('textbox', { name: 'Formato da resposta' })).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Tom de voz' })).toBeInTheDocument();
  });

  it('savesRuleOneSecondAfterTyping', async () => {
    const corpos: unknown[] = [];
    responderCom({
      ...rotasBase(),
      'PUT /api/arquivo': async (req: Request) => {
        corpos.push(await req.json());
        return json({ salvo: politicas });
      },
    });
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Politicas' });
    await waitFor(() => expect(campo).toBeEnabled());
    await user.type(campo, ' Sempre.');
    expect(corpos).toEqual([]);
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos).toEqual([{ agente: AGENTE, caminho: politicas, conteudo: 'Seja gentil. Sempre.' }]);
  });

  it('createsRule', async () => {
    const corpos: unknown[] = [];
    responderCom({
      ...rotasBase(),
      'PUT /api/arquivo': async (req: Request) => {
        corpos.push(await req.json());
        return json({ salvo: 'ok' });
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Nova regra' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Prazo de resposta');
    await user.click(screen.getByRole('button', { name: 'Criar' }));
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos).toEqual([{ agente: AGENTE, caminho: caminhoDeRegra('prazo-de-resposta.md'), conteudo: '' }]);
  });

  it('deletesRuleAfterConfirmation', async () => {
    const apagados: string[] = [];
    responderCom({
      ...rotasBase(),
      'DELETE /api/arquivo': (req: Request) => {
        apagados.push(new URL(req.url).searchParams.get('caminho') ?? '');
        return json({ excluido: politicas });
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Excluir Politicas' }));
    expect(screen.getByText('Excluir a regra "Politicas"? A skill deixa de segui-la.')).toBeInTheDocument();
    expect(apagados).toEqual([]);
    await user.click(screen.getByRole('button', { name: 'Excluir' }));
    await waitFor(() => expect(apagados).toEqual([politicas]));
  });
});
