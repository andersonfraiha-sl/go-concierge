import { focusManager } from '@tanstack/react-query';
import { act, fireEvent, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { LayoutDeDetalhe } from '@/app/detalhe/LayoutDeDetalhe';
import { estadoFalso } from '@/condominios/estadoFalso';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { SkillEditorPage } from './SkillEditorPage';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
  administradoraId: null,
  dev: false,
};
const gestor: Eu = { ...construtor, papel: 'gestor', administradoraId: 'adm-1' };

const lidos = (temFluxo = true) => ({
  caminho: 'skills/agendar-mudanca',
  versao: 'v1',
  textos: {
    titulo: 'Agendar mudança',
    quandoUsar: 'Quando o morador quer mudar.',
    frasesDeExemplo: ['Quero agendar uma mudança'],
    palavrasChave: ['mudança'],
    temFluxo,
    resumo: '',
    visibilidade: '',
  },
});

const montar = (rota = '/skills/agendar-mudanca') =>
  renderComProvedores(<SkillEditorPage />, { rota, caminho: '/skills/:nome' });

/** Rotas do editor; cada PUT dos textos devolve a próxima versão e guarda o corpo recebido. */
function rotasDoEditor(eu: Eu, corpos: unknown[], textos = lidos()) {
  return {
    'GET /api/eu': () => json(eu),
    'GET /api/skills': () => json({ assistente: 'concierge', skills: [] }),
    'GET /api/skills/agendar-mudanca': () => json(textos),
    // Sem fluxo gravado ainda nestes testes: a coluna "Informações que a skill pede" some.
    'GET /api/skills/agendar-mudanca/fluxo': () => json(null, 404),
    'GET /api/admin/estado': () => json(estadoFalso),
    'PUT /api/skills/agendar-mudanca/textos': async (req: Request) => {
      corpos.push(await req.json());
      return json({ versao: 'v2' });
    },
  };
}

describe('SkillEditorPage', () => {
  // shouldAdvanceTime mantém o relógio falso andando junto do real: interações do
  // userEvent continuam funcionando sem mudar de sentido; só o autosave de 1s
  // vira previsível com avanços explícitos abaixo, sem depender do relógio real.
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('savesSummaryOneSecondAfterTyping', async () => {
    const corpos: unknown[] = [];
    responderCom(rotasDoEditor(construtor, corpos));
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Resumo' });
    await waitFor(() => expect(campo).toBeEnabled());
    await user.type(campo, 'Agenda a mudança.');
    expect(corpos).toEqual([]);
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos).toEqual([
      { versao: 'v1', textos: { ...lidos().textos, resumo: 'Agenda a mudança.' } },
    ]);
  });

  /** A janela volta ao foco: a consulta refaz a leitura, como quando um colega editou. */
  const voltarAoFoco = () =>
    act(() => {
      focusManager.setFocused(false);
      focusManager.setFocused(true);
    });

  /** Resumo digitado e, antes de salvar, um colega grava outro resumo (versão v2). */
  async function conflitoComColega() {
    const corpos: unknown[] = [];
    let lido = lidos();
    responderCom({
      ...rotasDoEditor(construtor, corpos),
      'GET /api/skills/agendar-mudanca': () => json(lido),
    });
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Resumo' });
    await waitFor(() => expect(campo).toBeEnabled());
    fireEvent.change(campo, { target: { value: 'Meu resumo.' } });
    lido = { ...lidos(), versao: 'v2', textos: { ...lidos().textos, resumo: 'Do colega.' } };
    voltarAoFoco();

    expect(await screen.findByText(/Este campo mudou em outro lugar/)).toBeInTheDocument();
    await vi.advanceTimersByTimeAsync(2000);
    expect(corpos).toEqual([]);
    expect(campo).toHaveValue('Meu resumo.');
    return { user, campo, corpos };
  }

  it('keepsUserTextOverColleagueEditWhenChosen', async () => {
    const { user, corpos } = await conflitoComColega();

    await user.click(screen.getByRole('button', { name: 'Manter o meu' }));
    await vi.advanceTimersByTimeAsync(1000);

    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos).toEqual([
      { versao: 'v2', textos: { ...lidos().textos, resumo: 'Meu resumo.' } },
    ]);
  });

  it('usesColleagueVersionWhenChosen', async () => {
    const { user, campo, corpos } = await conflitoComColega();

    await user.click(screen.getByRole('button', { name: 'Usar a versão nova' }));
    await vi.advanceTimersByTimeAsync(2000);

    expect(campo).toHaveValue('Do colega.');
    expect(corpos).toEqual([]);
    expect(screen.queryByText(/Este campo mudou/)).not.toBeInTheDocument();
  });

  it('ignoresAnOlderReadThatArrivesAfterSaving', async () => {
    const corpos: unknown[] = [];
    let soltarLeitura = (_: Response) => {};
    let leituras = 0;
    responderCom({
      ...rotasDoEditor(construtor, corpos),
      'GET /api/skills/agendar-mudanca': () => {
        leituras += 1;
        if (leituras === 1) return json(lidos());
        return new Promise<Response>((resolver) => {
          soltarLeitura = resolver;
        });
      },
    });
    montar();
    const campo = await screen.findByRole('textbox', { name: 'Resumo' });
    await waitFor(() => expect(campo).toBeEnabled());
    fireEvent.change(campo, { target: { value: 'Novo.' } });
    voltarAoFoco();
    await waitFor(() => expect(leituras).toBe(2));
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(1));

    await act(async () => soltarLeitura(json(lidos())));

    expect(campo).toHaveValue('Novo.');
    expect(screen.queryByText(/Este campo mudou/)).not.toBeInTheDocument();
  });

  it('limitsSummaryTo90Characters', async () => {
    responderCom(rotasDoEditor(construtor, []));
    montar();
    const campo = await screen.findByRole('textbox', { name: 'Resumo' });
    expect(campo).toHaveAttribute('maxLength', '90');
    expect(screen.getByText('0/90')).toBeInTheDocument();
  });

  it('savesQuandoUsarOneSecondAfterTyping', async () => {
    const corpos: unknown[] = [];
    responderCom(rotasDoEditor(construtor, corpos));
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Como reconhecer o pedido' });
    await waitFor(() => expect(campo).toBeEnabled());
    await user.type(campo, ' Sempre.');
    expect(corpos).toEqual([]);
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos).toEqual([
      {
        versao: 'v1',
        textos: { ...lidos().textos, quandoUsar: 'Quando o morador quer mudar. Sempre.' },
      },
    ]);
  });

  it('renamesFromTitle', async () => {
    const corpos: unknown[] = [];
    responderCom(rotasDoEditor(construtor, corpos));
    // O "Salvo às" mora no cabeçalho de detalhe, que o layout de detalhe desenha.
    const { user } = renderComProvedores(<SkillEditorPage />, {
      rota: '/skills/agendar-mudanca',
      caminho: '/skills/:nome',
      layout: <LayoutDeDetalhe />,
    });
    await user.click(await screen.findByRole('button', { name: 'Editar Nome da skill' }));
    const campo = screen.getByRole('textbox', { name: 'Nome da skill' });
    await user.clear(campo);
    await user.type(campo, 'Mudança de unidade{Enter}');
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos).toEqual([
      { versao: 'v1', textos: { ...lidos().textos, titulo: 'Mudança de unidade' } },
    ]);
    expect(await screen.findByText(/Salvo às/)).toBeInTheDocument();
  });

  it('addsKeywordAndExamplePhrase', async () => {
    const corpos: unknown[] = [];
    responderCom(rotasDoEditor(construtor, corpos));
    const { user } = montar();
    await user.type(
      await screen.findByRole('textbox', { name: 'Nova palavra-chave' }),
      'elevador{Enter}',
    );
    await user.type(
      screen.getByRole('textbox', { name: 'Nova frase de exemplo' }),
      'Preciso do elevador{Enter}',
    );
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos).toEqual([
      {
        versao: 'v1',
        textos: {
          ...lidos().textos,
          palavrasChave: ['mudança', 'elevador'],
          frasesDeExemplo: ['Quero agendar uma mudança', 'Preciso do elevador'],
        },
      },
    ]);
  });

  it('dropsBlankPhrasesAndKeywordsBeforeSaving', async () => {
    const corpos: unknown[] = [];
    responderCom(
      rotasDoEditor(construtor, corpos, {
        ...lidos(),
        textos: { ...lidos().textos, frasesDeExemplo: ['Quero agendar uma mudança', '  '] },
      }),
    );
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Como reconhecer o pedido' });
    await waitFor(() => expect(campo).toBeEnabled());
    await user.type(campo, '!');
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos).toEqual([
      { versao: 'v1', textos: { ...lidos().textos, quandoUsar: 'Quando o morador quer mudar.!' } },
    ]);
  });

  it('showsNoKeywordsMessageWhenReadOnly', async () => {
    responderCom(
      rotasDoEditor(gestor, [], { ...lidos(), textos: { ...lidos().textos, palavrasChave: [] } }),
    );
    montar();
    expect(await screen.findByText('Nenhuma palavra-chave.')).toBeInTheDocument();
  });

  it('showsExamplesWithoutFlow', async () => {
    responderCom(rotasDoEditor(construtor, [], lidos(false)));
    montar();
    await screen.findByRole('textbox', { name: 'Como reconhecer o pedido' });
    expect(screen.getByText('Palavras-chave')).toBeInTheDocument();
    expect(screen.getByText('Frases de exemplo')).toBeInTheDocument();
  });

  it('gestorReadsOnly', async () => {
    responderCom(rotasDoEditor(gestor, []));
    montar();
    const campo = await screen.findByRole('textbox', { name: 'Como reconhecer o pedido' });
    expect(campo).toBeDisabled();
    expect(screen.getByText('“Quero agendar uma mudança”')).toBeInTheDocument();
    expect(
      screen.queryByRole('textbox', { name: 'Nova frase de exemplo' }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Editar Nome da skill' })).not.toBeInTheDocument();
  });

  it('opensTabFromQuery', async () => {
    responderCom({
      ...rotasDoEditor(construtor, []),
      'GET /api/agente': () =>
        json({
          agente: {
            caminho: 'skills/agendar-mudanca',
            nome: 'agendar-mudanca',
            descricao: '',
            persona: '',
            tipo: 'operador',
            tipo_declarado: false,
          },
          recursos: [],
        }),
    });
    montar('/skills/agendar-mudanca?aba=regras');
    expect(await screen.findByRole('tab', { name: 'Regras', selected: true })).toBeInTheDocument();
    expect(screen.getAllByRole('tab').map((aba) => aba.textContent)).toEqual([
      'Quando usar',
      'Fluxo',
      'Regras',
      'Base de conhecimento',
      'Ferramentas',
    ]);
  });
});
