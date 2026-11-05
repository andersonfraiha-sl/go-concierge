import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider, useLocation } from 'react-router';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { CopilotoProvider } from '@/copiloto/CopilotoProvider';
import { PainelDoCopiloto } from '@/copiloto/PainelDoCopiloto';
import { SnackbarProvider } from '@/shared/snackbar/SnackbarContext';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { criarTema } from '@/theme/theme';
import { SkillEditorPage } from './SkillEditorPage';
import { SkillsPage } from './SkillsPage';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos', 'skills.criar', 'skills.excluir'],
  administradoraId: null,
  dev: false,
};
const gestor: Eu = {
  ...construtor,
  papel: 'gestor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
};
const construtorComFluxo: Eu = {
  ...construtor,
  permissoes: [...construtor.permissoes, 'skills.editar_fluxo'],
};

const mudanca = {
  nome: 'agendar-mudanca',
  id: 'agendar-mudanca',
  titulo: 'Agendar mudança',
  descricao: 'Reserva o elevador para a mudança.',
  resumo: 'Reserva o elevador para a mudança.',
  icone: 'truck',
  escrita: true,
  tipo: 'fluxo',
  recursos: ['reservar', 'listar'],
  caminho: 'skills/agendar-mudanca',
};
const faq: typeof mudanca = {
  ...mudanca,
  nome: 'faq',
  id: 'faq',
  titulo: 'Perguntas frequentes',
  resumo: 'Tira dúvidas comuns dos moradores.',
  icone: '',
  escrita: false,
  recursos: ['buscar'],
  caminho: 'skills/faq',
};

const noDaArvore = (caminho: string, erros: number, avisos: number) => ({
  caminho,
  nome: caminho,
  tipo: 'skill',
  descricao: '',
  caso: true,
  erros,
  avisos,
  subagentes: [],
});

const lista = (skills: (typeof mudanca)[]) => json({ assistente: 'concierge', skills });

function Local() {
  return <output aria-label="rota">{useLocation().pathname}</output>;
}

function montar() {
  return renderComProvedores(
    <>
      <SkillsPage />
      <Local />
    </>,
    { rota: '/skills' },
  );
}

describe('SkillsPage', () => {
  it('listsSkillCards', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([mudanca, faq]),
    });
    montar();
    const cartao = await screen.findByRole('article', { name: 'Agendar mudança' });
    expect(within(cartao).getByText('Reserva o elevador para a mudança.')).toBeInTheDocument();
    const outro = screen.getByRole('article', { name: 'Perguntas frequentes' });
    expect(within(outro).getByText('Tira dúvidas comuns dos moradores.')).toBeInTheDocument();
  });

  it('showsFourStatCards', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([mudanca, faq]),
    });
    montar();
    await screen.findByRole('article', { name: 'Agendar mudança' });
    expect(
      within(screen.getByRole('group', { name: 'Skills' })).getByText('2'),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('group', { name: 'Registram no sistema' })).getByText('1'),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('group', { name: 'Consultas e dúvidas' })).getByText('1'),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('group', { name: 'Ferramentas' })).getByText('3'),
    ).toBeInTheDocument();
  });

  it('groupsWritingSkillsFirst', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq, mudanca]),
    });
    montar();
    await screen.findByRole('article', { name: 'Agendar mudança' });
    const titulosDosGrupos = screen.getAllByRole('heading', { level: 2 }).map((h) => h.textContent);
    expect(titulosDosGrupos).toEqual(['Registram no sistema', 'Consultas e dúvidas']);
  });

  it('showsSummaryAndIcon', async () => {
    const comResumoProprio = { ...mudanca, resumo: 'Marca a data da mudança.' };
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([comResumoProprio]),
    });
    montar();
    const cartao = await screen.findByRole('article', { name: 'Agendar mudança' });
    expect(within(cartao).getByText('Marca a data da mudança.')).toBeInTheDocument();
    expect(within(cartao).queryByText(mudanca.descricao)).not.toBeInTheDocument();
    expect(cartao.querySelector('svg')).not.toBeNull();
  });

  it('showsProblemCountForBuilder', async () => {
    responderCom({
      'GET /api/eu': () => json(construtorComFluxo),
      'GET /api/skills': () => lista([mudanca, faq]),
      'GET /api/agentes': () =>
        json({ agentes: [noDaArvore(mudanca.caminho, 1, 0), noDaArvore(faq.caminho, 0, 0)] }),
    });
    montar();
    const cartao = await screen.findByRole('article', { name: 'Agendar mudança' });
    expect(await within(cartao).findByText('1 erro')).toBeInTheDocument();
    expect(
      within(screen.getByRole('group', { name: 'Com problema' })).getByText('1'),
    ).toBeInTheDocument();
  });

  it('hidesProblemCountForGestor', async () => {
    const fetchFalso = responderCom({
      'GET /api/eu': () => json(gestor),
      'GET /api/skills': () => lista([mudanca, faq]),
    });
    montar();
    await screen.findByRole('article', { name: 'Agendar mudança' });
    expect(screen.getByRole('group', { name: 'Ferramentas' })).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Com problema' })).not.toBeInTheDocument();
    expect(screen.queryByText('1 erro')).not.toBeInTheDocument();
    const chamadas = fetchFalso.mock.calls.map(([entrada]) => String(entrada));
    expect(chamadas.some((url) => url.includes('/api/agentes'))).toBe(false);
  });

  it('showsLoadingInsteadOfZeroWhileTreeLoads', async () => {
    responderCom({
      'GET /api/eu': () => json(construtorComFluxo),
      'GET /api/skills': () => lista([mudanca, faq]),
      'GET /api/agentes': () => new Promise<Response>(() => {}),
    });
    montar();
    await screen.findByRole('article', { name: 'Agendar mudança' });
    const cartao = screen.getByRole('group', { name: 'Com problema' });
    expect(within(cartao).getByLabelText('Carregando problemas')).toBeInTheDocument();
    expect(within(cartao).queryByText('0')).not.toBeInTheDocument();
  });

  it('offersRetryWhenTreeFails', async () => {
    responderCom({
      'GET /api/eu': () => json(construtorComFluxo),
      'GET /api/skills': () => lista([mudanca, faq]),
      'GET /api/agentes': () => json({ erro: 'falhou' }, 500),
    });
    montar();
    expect(await screen.findByRole('button', { name: 'Tentar de novo' })).toBeInTheDocument();
    expect(screen.queryByRole('group', { name: 'Com problema' })).not.toBeInTheDocument();
  });

  it('showsEmptyState', async () => {
    responderCom({ 'GET /api/eu': () => json(construtor), 'GET /api/skills': () => lista([]) });
    montar();
    expect(
      await screen.findByText(
        'Uma skill é algo que o concierge sabe fazer pelo morador, como reservar o salão ou mandar a 2ª via do boleto.',
      ),
    ).toBeInTheDocument();
  });

  it('offersCopilotSuggestionsOnEmptyHub', async () => {
    let enviado: unknown = null;
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([]),
      'GET /api/copiloto/estado': () => json({ disponivel: true }),
      'POST /api/copiloto': async (req) => {
        enviado = await req.json();
        return json({ mensagem: 'Vamos lá.', propostas: [], recusas: [] });
      },
    });
    const { user } = renderComProvedores(
      <>
        <PainelDoCopiloto />
        <SkillsPage />
      </>,
      { rota: '/skills' },
    );
    const sugestoes = await screen.findByRole('group', { name: 'Ou peça ao Copiloto' });
    expect(
      within(sugestoes)
        .getAllByRole('button')
        .map((chip) => chip.textContent),
    ).toEqual([
      'Crie uma skill de 2ª via de boleto',
      'Crie uma skill de agendamento de mudança',
      'Crie uma skill para reservar o salão',
    ]);
    expect(screen.queryByRole('complementary', { name: 'Copiloto' })).not.toBeInTheDocument();

    await user.click(
      within(sugestoes).getByRole('button', { name: 'Crie uma skill de 2ª via de boleto' }),
    );

    expect(await screen.findByRole('complementary', { name: 'Copiloto' })).toBeInTheDocument();
    expect(await screen.findByText('Vamos lá.')).toBeInTheDocument();
    expect(enviado).toMatchObject({ mensagem: 'Crie uma skill de 2ª via de boleto' });
  });

  it('hidesCopilotSuggestionsWithoutCreatePermission', async () => {
    responderCom({ 'GET /api/eu': () => json(gestor), 'GET /api/skills': () => lista([]) });
    montar();
    await screen.findByText(/Uma skill é algo que o concierge sabe fazer/);
    expect(screen.queryByRole('group', { name: 'Ou peça ao Copiloto' })).not.toBeInTheDocument();
  });

  it('offersRetryWhenListFails', async () => {
    let falhas = 1;
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () =>
        falhas-- > 0 ? json({ codigo: 'X', mensagem: 'Servidor caiu.' }, 500) : lista([faq]),
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Tentar de novo' }));
    expect(
      await screen.findByRole('article', { name: 'Perguntas frequentes' }),
    ).toBeInTheDocument();
  });

  it('createsSkillWithSummaryAndWhenToUse', async () => {
    const corposCriar: unknown[] = [];
    const corposTextos: unknown[] = [];
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'POST /api/skills': async (req) => {
        corposCriar.push(await req.json());
        return json({ nome: 'reserva-do-salao' }, 201);
      },
      'GET /api/skills/reserva-do-salao': () =>
        json({
          caminho: 'skills/reserva-do-salao',
          versao: 'v1',
          textos: {
            titulo: 'Reserva do Salão',
            quandoUsar: '',
            frasesDeExemplo: [],
            palavrasChave: [],
            temFluxo: false,
            resumo: '',
            visibilidade: '',
          },
        }),
      'PUT /api/skills/reserva-do-salao/textos': async (req) => {
        corposTextos.push(await req.json());
        return json({ versao: 'v2' });
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Nova skill' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Reserva do Salão');
    await user.type(screen.getByRole('textbox', { name: 'Resumo' }), 'Reserva o salão de festas.');
    await user.type(
      screen.getByRole('textbox', { name: 'Como reconhecer o pedido' }),
      'O morador quer reservar o salão.',
    );
    expect(screen.getByText('Identificador: reserva-do-salao')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Criar' }));
    expect(await screen.findByText('Skill criada.')).toBeInTheDocument();
    expect(corposCriar).toEqual([{ nome: 'reserva-do-salao', titulo: 'Reserva do Salão' }]);
    expect(corposTextos).toEqual([
      {
        versao: 'v1',
        textos: {
          titulo: 'Reserva do Salão',
          quandoUsar: 'O morador quer reservar o salão.',
          frasesDeExemplo: [],
          palavrasChave: [],
          temFluxo: false,
          resumo: 'Reserva o salão de festas.',
          visibilidade: '',
        },
      },
    ]);
    await waitFor(() =>
      expect(screen.getByLabelText('rota')).toHaveTextContent('/skills/reserva-do-salao'),
    );
  });

  const rotasDaCriacaoComSkillJaCriada = (corposCriar: unknown[], aoChamarPut: () => Response) => ({
    'GET /api/eu': () => json(construtor),
    'GET /api/skills': () => lista([faq]),
    'POST /api/skills': async (req: Request) => {
      corposCriar.push(await req.json());
      return json({ nome: 'reserva-do-salao' }, 201);
    },
    'GET /api/skills/reserva-do-salao': () =>
      json({
        caminho: 'skills/reserva-do-salao',
        versao: 'v1',
        textos: {
          titulo: 'Reserva do Salão',
          quandoUsar: '',
          frasesDeExemplo: [],
          palavrasChave: [],
          temFluxo: false,
          resumo: '',
          visibilidade: '',
        },
      }),
    'PUT /api/skills/reserva-do-salao/textos': () => aoChamarPut(),
  });

  it('showsFormMessageWhenTextsFailToSaveAfterCreating', async () => {
    const corposCriar: unknown[] = [];
    let tentativasPut = 0;
    responderCom(
      rotasDaCriacaoComSkillJaCriada(corposCriar, () => {
        tentativasPut += 1;
        return json({ codigo: 'X', mensagem: 'Servidor caiu.' }, 500);
      }),
    );
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Nova skill' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Reserva do Salão');
    await user.type(screen.getByRole('textbox', { name: 'Resumo' }), 'Reserva o salão de festas.');
    await user.click(screen.getByRole('button', { name: 'Criar' }));

    expect(
      await screen.findByText('A skill foi criada, mas os textos não salvaram. Tente de novo.'),
    ).toBeInTheDocument();
    const nome = screen.getByRole('textbox', { name: 'Nome' });
    expect(nome).toHaveAttribute('aria-invalid', 'false');
    expect(screen.queryByText('Use letras ou números no nome.')).not.toBeInTheDocument();
    expect(corposCriar).toHaveLength(1);
    expect(tentativasPut).toBe(1);
    expect(screen.getByLabelText('rota')).toHaveTextContent('/skills');
  });

  it('retrySendsOnlyThePutNotThePost', async () => {
    const corposCriar: unknown[] = [];
    let tentativasPut = 0;
    responderCom(
      rotasDaCriacaoComSkillJaCriada(corposCriar, () => {
        tentativasPut += 1;
        return tentativasPut === 1
          ? json({ codigo: 'X', mensagem: 'Servidor caiu.' }, 500)
          : json({ versao: 'v2' });
      }),
    );
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Nova skill' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Reserva do Salão');
    await user.type(screen.getByRole('textbox', { name: 'Resumo' }), 'Reserva o salão de festas.');
    await user.click(screen.getByRole('button', { name: 'Criar' }));
    await screen.findByText('A skill foi criada, mas os textos não salvaram. Tente de novo.');

    await user.click(screen.getByRole('button', { name: 'Tentar de novo' }));

    await waitFor(() =>
      expect(screen.getByLabelText('rota')).toHaveTextContent('/skills/reserva-do-salao'),
    );
    expect(corposCriar).toHaveLength(1);
    expect(tentativasPut).toBe(2);
  });

  /** Router real com as duas páginas: a navegação do "Fechar" leva de fato ao editor. */
  function montarComEditor() {
    const cliente = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const rotas = createMemoryRouter(
      [
        { path: '/skills', element: <SkillsPage /> },
        { path: '/skills/:nome', element: <SkillEditorPage /> },
      ],
      { initialEntries: ['/skills'] },
    );
    render(
      <QueryClientProvider client={cliente}>
        <ThemeProvider theme={criarTema()}>
          <CssBaseline />
          <SnackbarProvider>
            <CopilotoProvider>
              <RouterProvider router={rotas} />
            </CopilotoProvider>
          </SnackbarProvider>
        </ThemeProvider>
      </QueryClientProvider>,
    );
    return { user: userEvent.setup() };
  }

  it('sendsPendingTextsFromEditorAfterClosingOnFailure', async () => {
    const corposCriar: unknown[] = [];
    const corposTextos: unknown[] = [];
    let tentativasPut = 0;
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'POST /api/skills': async (req) => {
        corposCriar.push(await req.json());
        return json({ nome: 'reserva-do-salao' }, 201);
      },
      'GET /api/skills/reserva-do-salao': () =>
        json({
          caminho: 'skills/reserva-do-salao',
          versao: 'v1',
          textos: {
            titulo: 'Reserva do Salão',
            quandoUsar: '',
            frasesDeExemplo: [],
            palavrasChave: [],
            temFluxo: false,
            resumo: '',
            visibilidade: '',
          },
        }),
      'PUT /api/skills/reserva-do-salao/textos': async (req) => {
        tentativasPut += 1;
        if (tentativasPut === 1) return json({ codigo: 'X', mensagem: 'Servidor caiu.' }, 500);
        corposTextos.push(await req.json());
        return json({ versao: 'v2' });
      },
    });
    const { user } = montarComEditor();

    await user.click(await screen.findByRole('button', { name: 'Nova skill' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Reserva do Salão');
    await user.type(screen.getByRole('textbox', { name: 'Resumo' }), 'Reserva o salão de festas.');
    await user.type(
      screen.getByRole('textbox', { name: 'Como reconhecer o pedido' }),
      'O morador quer reservar o salão.',
    );
    await user.click(screen.getByRole('button', { name: 'Criar' }));
    await screen.findByText('A skill foi criada, mas os textos não salvaram. Tente de novo.');

    await user.click(screen.getByRole('button', { name: 'Fechar' }));

    await screen.findByRole('textbox', { name: 'Resumo' }); // já no editor, sem tocar em nada
    await waitFor(() => expect(corposTextos).toHaveLength(1), { timeout: 5000 });
    expect(corposTextos).toEqual([
      {
        versao: 'v1',
        textos: {
          titulo: 'Reserva do Salão',
          quandoUsar: 'O morador quer reservar o salão.',
          frasesDeExemplo: [],
          palavrasChave: [],
          temFluxo: false,
          resumo: 'Reserva o salão de festas.',
          visibilidade: '',
        },
      },
    ]);
  });

  it('duplicatesSkillStartingFromCopyName', async () => {
    const corpos: unknown[] = [];
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'POST /api/skills/faq/duplicar': async (req) => {
        corpos.push(await req.json());
        return json({ nome: 'copia-de-perguntas-frequentes' }, 201);
      },
    });
    const { user } = montar();
    await user.click(
      await screen.findByRole('button', { name: 'Mais ações de Perguntas frequentes' }),
    );
    await user.click(screen.getByRole('menuitem', { name: 'Duplicar Perguntas frequentes' }));
    expect(screen.getByRole('textbox', { name: 'Nome' })).toHaveValue(
      'Cópia de Perguntas frequentes',
    );
    await user.click(screen.getByRole('button', { name: 'Duplicar' }));
    expect(await screen.findByText('Skill duplicada.')).toBeInTheDocument();
    expect(corpos).toEqual([
      { nome: 'copia-de-perguntas-frequentes', titulo: 'Cópia de Perguntas frequentes' },
    ]);
  });

  it('showsNameErrorFromServer', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'POST /api/skills': () =>
        json({ codigo: 'SKILL_JA_EXISTE', mensagem: 'Já existe uma skill com esse nome.' }, 409),
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Nova skill' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Faq');
    await user.click(screen.getByRole('button', { name: 'Criar' }));
    expect(await screen.findByText('Já existe uma skill com esse nome.')).toBeInTheDocument();
    expect(screen.getByLabelText('rota')).toHaveTextContent('/skills');
  });

  it('deletesAfterConfirmation', async () => {
    const chamadas: string[] = [];
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'DELETE /api/skills/faq': () => {
        chamadas.push('DELETE');
        return new Response(null, { status: 204 });
      },
    });
    const { user } = montar();
    await user.click(
      await screen.findByRole('button', { name: 'Mais ações de Perguntas frequentes' }),
    );
    await user.click(screen.getByRole('menuitem', { name: 'Excluir Perguntas frequentes' }));
    expect(
      screen.getByText('Excluir "Perguntas frequentes"? O fluxo e as regras somem junto.'),
    ).toBeInTheDocument();
    expect(chamadas).toEqual([]);
    await user.click(screen.getByRole('button', { name: 'Excluir' }));
    expect(await screen.findByText('Skill excluída.')).toBeInTheDocument();
    expect(chamadas).toEqual(['DELETE']);
  });

  it('gestorSeesNoStructureButtons', async () => {
    responderCom({ 'GET /api/eu': () => json(gestor), 'GET /api/skills': () => lista([faq]) });
    montar();
    await screen.findByRole('article', { name: 'Perguntas frequentes' });
    expect(screen.getByRole('button', { name: 'Abrir Perguntas frequentes' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Nova skill' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Duplicar/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Excluir/ })).not.toBeInTheDocument();
  });
});

describe('SkillsPage em tela estreita', () => {
  it('narrowScreenHidesStructureButtons', async () => {
    const original = window.matchMedia;
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: (query: string) => ({ ...original(query), matches: true }),
    });
    try {
      responderCom({
        'GET /api/eu': () => json(construtor),
        'GET /api/skills': () => lista([faq]),
      });
      montar();
      await screen.findByRole('article', { name: 'Perguntas frequentes' });
      expect(
        screen.getByRole('button', { name: 'Abrir Perguntas frequentes' }),
      ).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Nova skill' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Duplicar/ })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Excluir/ })).not.toBeInTheDocument();
    } finally {
      Object.defineProperty(window, 'matchMedia', {
        configurable: true,
        writable: true,
        value: original,
      });
    }
  });
});
