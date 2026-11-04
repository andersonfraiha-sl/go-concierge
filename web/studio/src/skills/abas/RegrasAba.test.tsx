import { screen, waitFor, within } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { caminhoDeRegra } from '@/catalogo/contratos';
import { digitarNoFim } from '@/test/editorRico';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { AGENTE, agenteLido, arquivo, construtor, gestor } from './fixturesDeAbas';
import { RegrasAba } from './RegrasAba';

const politicas = caminhoDeRegra('politicas.md');

const montar = () => renderComProvedores(<RegrasAba nome="agendar-mudanca" caminho={AGENTE} />);

function Alternavel() {
  const [visivel, setVisivel] = useState(true);
  return (
    <>
      <button type="button" onClick={() => setVisivel((v) => !v)}>
        Alternar
      </button>
      {visivel && <RegrasAba nome="agendar-mudanca" caminho={AGENTE} />}
    </>
  );
}

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
    expect(await screen.findByRole('textbox', { name: 'Politicas' })).toHaveTextContent(
      'Seja gentil.',
    );
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
    await waitFor(() => expect(campo).toHaveAttribute('aria-readonly', 'false'));
    await digitarNoFim(user, campo, ' Sempre.');
    expect(corpos).toEqual([]);
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos).toEqual([
      { agente: AGENTE, caminho: politicas, conteudo: 'Seja gentil. Sempre.' },
    ]);
  });

  it('savesToolbarFormattingAsMarkdown', async () => {
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
    await waitFor(() => expect(campo).toHaveAttribute('aria-readonly', 'false'));
    const barra = screen.getByRole('toolbar', { name: 'Formatação de Politicas' });
    await user.click(within(barra).getByRole('button', { name: 'Lista com marcadores' }));
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos).toEqual([{ agente: AGENTE, caminho: politicas, conteudo: '- Seja gentil.' }]);
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
    expect(corpos).toEqual([
      { agente: AGENTE, caminho: caminhoDeRegra('prazo-de-resposta.md'), conteudo: '' },
    ]);
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
    expect(
      screen.getByText('Excluir a regra "Politicas"? A skill deixa de segui-la.'),
    ).toBeInTheDocument();
    expect(apagados).toEqual([]);
    await user.click(screen.getByRole('button', { name: 'Excluir' }));
    await waitFor(() => expect(apagados).toEqual([politicas]));
  });

  it('keepsSavedTextWhenEditorRemounts', async () => {
    let conteudo = 'Seja gentil.';
    const corpos: { conteudo: string }[] = [];
    responderCom({
      ...rotasBase(),
      'GET /api/arquivo': () => json({ caminho: politicas, conteudo }),
      'PUT /api/arquivo': async (req: Request) => {
        const corpo: { conteudo: string } = await req.json();
        corpos.push(corpo);
        conteudo = corpo.conteudo;
        return json({ salvo: politicas });
      },
    });
    const { user } = renderComProvedores(<Alternavel />);
    const primeiro = await screen.findByRole('textbox', { name: 'Politicas' });
    await waitFor(() => expect(primeiro).toHaveAttribute('aria-readonly', 'false'));
    await digitarNoFim(user, primeiro, ' A');
    expect(await screen.findByText(/Salvo às/, undefined, { timeout: 3000 })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Alternar' }));
    await user.click(screen.getByRole('button', { name: 'Alternar' }));
    const segundo = await screen.findByRole('textbox', { name: 'Politicas' });
    await waitFor(() => expect(segundo).toHaveAttribute('aria-readonly', 'false'));
    expect(segundo).toHaveTextContent('Seja gentil. A');
    await digitarNoFim(user, segundo, ' B');
    await waitFor(() => expect(corpos).toHaveLength(2), { timeout: 3000 });
    expect(corpos[1]?.conteudo).toBe('Seja gentil. A B');
  });

  it('dropsPendingSaveWhenRuleIsDeleted', async () => {
    let existe = true;
    const metodos: string[] = [];
    responderCom({
      ...rotasBase(),
      'GET /api/agente': () => json(agenteLido({ regras: existe ? [arquivo(politicas)] : [] })),
      'PUT /api/arquivo': () => {
        metodos.push('PUT');
        return json({ salvo: politicas });
      },
      'DELETE /api/arquivo': () => {
        existe = false;
        metodos.push('DELETE');
        return json({ excluido: politicas });
      },
    });
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Politicas' });
    await waitFor(() => expect(campo).toHaveAttribute('aria-readonly', 'false'));
    await digitarNoFim(user, campo, ' pendente');
    await user.click(screen.getByRole('button', { name: 'Excluir Politicas' }));
    await user.click(screen.getByRole('button', { name: 'Excluir' }));
    await waitFor(() =>
      expect(screen.queryByRole('textbox', { name: 'Politicas' })).not.toBeInTheDocument(),
    );
    await new Promise((resolve) => setTimeout(resolve, 1500));
    expect(metodos).toEqual(['DELETE']);
  });

  it('retriesAfterLoadError', async () => {
    let falhar = true;
    responderCom({
      ...rotasBase(),
      'GET /api/agente': () =>
        falhar
          ? json({ codigo: 'ERRO', mensagem: 'boom' }, 500)
          : json(agenteLido({ regras: [arquivo(politicas)] })),
    });
    const { user } = montar();
    const tentar = await screen.findByRole('button', { name: 'Tentar de novo' });
    falhar = false;
    await user.click(tentar);
    expect(await screen.findByRole('textbox', { name: 'Politicas' })).toBeInTheDocument();
  });

  it('gestorReadsOnly', async () => {
    responderCom({ ...rotasBase(), 'GET /api/eu': () => json(gestor) });
    montar();
    const campo = await screen.findByRole('textbox', { name: 'Politicas' });
    expect(campo).toHaveAttribute('aria-readonly', 'true');
    expect(screen.queryByRole('button', { name: 'Nova regra' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Excluir Politicas' })).not.toBeInTheDocument();
  });

  it('titledRegrasGeraisForConcierge', async () => {
    responderCom(rotasBase());
    renderComProvedores(<RegrasAba caminho={AGENTE} titulo="Regras gerais" />);
    expect(await screen.findByRole('heading', { name: 'Regras gerais' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Regras' })).not.toBeInTheDocument();
    expect(await screen.findByRole('textbox', { name: 'Politicas' })).toHaveTextContent(
      'Seja gentil.',
    );
  });
});
