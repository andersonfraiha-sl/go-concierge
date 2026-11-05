import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { useState } from 'react';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Eu } from '@/acesso/eu';
import fixture from '@/fluxo/modelo/agendar-mudanca.fixture.json';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { SOMBRA_DO_BLOCO_EM_EDICAO, tokens } from '@/theme/tokens';
import { FluxoAba } from './FluxoAba';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_fluxo'],
  administradoraId: null,
  dev: false,
};
const gestor: Eu = { ...construtor, papel: 'gestor', administradoraId: 'adm-1' };

const ferramentas = {
  account: 'conta',
  origem: 'nexus',
  conferido: true,
  recursos: [
    {
      name: 'unidades.listar',
      titulo: 'Listar unidades',
      kind: 'api',
      visibility: 'private',
      write: false,
      description: '',
    },
    {
      name: 'mudancas.agendar',
      titulo: 'Agendar mudança',
      kind: 'api',
      visibility: 'private',
      write: true,
      description: '',
    },
  ],
};

const CONFLITO = 'Esta skill mudou desde que você abriu. Recarregue para continuar.';

type Rota = (req: Request) => Response | Promise<Response>;

function rotas(eu: Eu, extras: Record<string, Rota> = {}): Record<string, Rota> {
  return {
    'GET /api/eu': () => json(eu),
    'GET /api/skills/agendar-mudanca/fluxo': () => json(fixture),
    'GET /api/nexus/recursos': () => json(ferramentas),
    ...extras,
  };
}

/** PUT que guarda o corpo recebido e responde com a próxima versão. */
function putQueGuarda(
  corpos: unknown[],
  resposta: () => Response = () => json({ versao: 'v2', problemas: [] }),
): Rota {
  return async (req) => {
    corpos.push(await req.json());
    return resposta();
  };
}

const montar = () =>
  renderComProvedores(<FluxoAba nome="agendar-mudanca" caminho="skills/agendar-mudanca" />);

/** Simula a troca de aba do editor: a aba sai da tela e volta, com o mesmo cache de consultas. */
function ComTrocaDeAba() {
  const [visivel, setVisivel] = useState(true);
  return (
    <>
      <button type="button" onClick={() => setVisivel(!visivel)}>
        Trocar de aba
      </button>
      {visivel && <FluxoAba nome="agendar-mudanca" caminho="skills/agendar-mudanca" />}
    </>
  );
}

/** Ao sair da tela com a gravação falhada, o autosave tenta de novo e registra o erro: é esperado. */
function silenciarGravacaoAoSair() {
  const erros = vi.spyOn(console, 'error').mockImplementation(() => {});
  return async (sair: () => void) => {
    sair();
    await waitFor(() =>
      expect(erros).toHaveBeenCalledWith('salvar ao sair da tela', expect.anything()),
    );
    erros.mockRestore();
  };
}

const rotulosDosBlocos = (): (string | null)[] =>
  within(screen.getByRole('region', { name: 'Fluxo' }))
    .getAllByRole('article')
    .map((bloco) => bloco.getAttribute('aria-label'));

const bloco = (rotulo: string): Promise<HTMLElement> =>
  screen.findByRole('article', { name: rotulo });

/** Clique sem mousedown: o d3-drag do nó arrastável lê `event.view`, que o user-event deixa nulo no jsdom. */
const selecionar = async (rotulo: string): Promise<void> => {
  fireEvent.click(await bloco(rotulo));
};

/** Avança o relógio falso além do atraso de 1s do autosave, sem depender do relógio real. */
const esperarUmSegundoESobra = () => vi.advanceTimersByTimeAsync(1300);

/** O React Flow só mede os nós no callback do ResizeObserver; sem medida, nem nós visíveis nem arestas. */
function medirNoJsdom(): void {
  class ResizeObserverQueMede {
    constructor(private readonly callback: ResizeObserverCallback) {}
    observe(target: Element): void {
      const contentRect = {
        x: 0,
        y: 0,
        top: 0,
        left: 0,
        width: 800,
        height: 600,
        bottom: 600,
        right: 800,
      };
      // Depois do commit: o React Flow só guarda o DOM do canvas no efeito do pai, que roda após o dos nós.
      setTimeout(() => {
        if (target.isConnected)
          this.callback(
            [{ target, contentRect } as ResizeObserverEntry],
            this as unknown as ResizeObserver,
          );
      }, 0);
    }
    unobserve(): void {}
    disconnect(): void {}
  }
  for (const alvo of [globalThis, window]) {
    Object.defineProperty(alvo, 'ResizeObserver', {
      value: ResizeObserverQueMede,
      configurable: true,
    });
  }
  Object.defineProperties(HTMLElement.prototype, {
    offsetHeight: { configurable: true, get: () => 600 },
    offsetWidth: { configurable: true, get: () => 800 },
  });
}

describe('FluxoAba', () => {
  beforeAll(medirNoJsdom);
  // shouldAdvanceTime mantém o relógio falso andando junto do real: interações do
  // userEvent e o ResizeObserver mockado continuam funcionando sem mudar de sentido;
  // só o autosave vira previsível com avanços explícitos abaixo.
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('rendersOneBlockPerStepInOrder', async () => {
    responderCom(rotas(construtor));
    const { container } = montar();
    await bloco('Responder');
    expect(rotulosDosBlocos()).toEqual([
      'Buscar dados',
      'Pedir escolha',
      'Perguntar',
      'Confirmar',
      'Gravar no sistema',
      'Responder',
    ]);
    expect(within(await bloco('Buscar dados')).getByText('Listar unidades')).toBeInTheDocument();
    await waitFor(() =>
      expect(container.querySelectorAll('.react-flow__node.draggable')).toHaveLength(5),
    );
  });

  it('insertsBlockFromPlusBetweenBlocks', async () => {
    responderCom(rotas(construtor));
    const { user } = montar();
    await bloco('Responder');
    const [entrePrimeiroESegundo] = await screen.findAllByRole('button', {
      name: 'Adicionar bloco aqui',
    });
    if (!entrePrimeiroESegundo) throw new Error('sem + entre blocos');
    await user.click(entrePrimeiroESegundo);
    await user.click(screen.getByRole('menuitem', { name: /^Confirmar/ }));
    await waitFor(() => expect(rotulosDosBlocos()).toHaveLength(7));
    expect(rotulosDosBlocos()[1]).toBe('Confirmar');
    const painel = screen.getByRole('region', { name: 'Bloco selecionado' });
    expect(within(painel).getByRole('heading', { name: 'Confirmar' })).toBeInTheDocument();
    expect(within(painel).getByRole('textbox', { name: 'Pergunta de confirmação' })).toHaveValue(
      '',
    );
  });

  it('marksGravarWithoutConfirmar', async () => {
    const corpos: unknown[] = [];
    responderCom(
      rotas(construtor, { 'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda(corpos) }),
    );
    const { user } = montar();
    await selecionar('Confirmar');
    await user.click(screen.getByRole('button', { name: 'Excluir bloco' }));
    const dialogo = screen.getByRole('dialog', { name: 'Excluir o bloco "Confirmar"?' });
    await user.click(within(dialogo).getByRole('button', { name: 'Excluir' }));

    expect(
      await screen.findByLabelText('Erro: Coloque um Confirmar antes de gravar no sistema.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Não salvo: corrija o bloco marcado')).toBeInTheDocument();
    await selecionar('Gravar no sistema');
    expect(screen.getByText('Coloque um Confirmar antes de gravar no sistema.')).toBeVisible();
    await esperarUmSegundoESobra();
    expect(corpos).toEqual([]);
  });

  it('savesOneSecondAfterEdit', async () => {
    const corpos: unknown[] = [];
    responderCom(
      rotas(construtor, { 'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda(corpos) }),
    );
    const { user } = montar();
    await selecionar('Confirmar');
    const campo = screen.getByRole('textbox', { name: 'Pergunta de confirmação' });
    await user.clear(campo);
    await user.type(campo, 'Posso agendar?');
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toMatchObject({
      versao: fixture.versao,
      fluxo: {
        blocos: expect.arrayContaining([
          expect.objectContaining({ id: 'b4', texto: 'Posso agendar?' }),
        ]),
      },
    });
    expect(await screen.findByText(/Salvo às/)).toBeInTheDocument();
  });

  it('showsServerRejection', async () => {
    const rejeicao = () =>
      json(
        {
          codigo: 'FLUXO_INVALIDO',
          mensagem: 'O fluxo tem problemas.',
          problemas: [
            {
              blocoId: 'b5',
              codigo: 'RECURSO_RECUSADO',
              nivel: 'erro',
              mensagem: 'A ferramenta recusou os dados.',
            },
          ],
        },
        422,
      );
    responderCom(
      rotas(construtor, { 'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda([], rejeicao) }),
    );
    const conferirSaida = silenciarGravacaoAoSair();
    const { user, unmount } = montar();
    await selecionar('Confirmar');
    await user.type(screen.getByRole('textbox', { name: 'Pergunta de confirmação' }), '!');
    await vi.advanceTimersByTimeAsync(1000);
    expect(await screen.findByText('Falha ao salvar')).toBeInTheDocument();
    expect(
      within(await bloco('Gravar no sistema')).getByLabelText(
        'Erro: A ferramenta recusou os dados.',
      ),
    ).toBeInTheDocument();
    await conferirSaida(unmount);
  });

  it('showsConflictAndKeepsEdit', async () => {
    const conflito = () => json({ codigo: 'VERSAO_ANTIGA', mensagem: CONFLITO }, 409);
    responderCom(
      rotas(construtor, { 'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda([], conflito) }),
    );
    const conferirSaida = silenciarGravacaoAoSair();
    const { user, unmount } = montar();
    await selecionar('Confirmar');
    const campo = screen.getByRole('textbox', { name: 'Pergunta de confirmação' });
    await user.clear(campo);
    await user.type(campo, 'Posso agendar?');
    await vi.advanceTimersByTimeAsync(1000);
    expect(await screen.findByText('Falha ao salvar')).toBeInTheDocument();
    expect(screen.getByLabelText(CONFLITO)).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Pergunta de confirmação' })).toHaveValue(
      'Posso agendar?',
    );
    await conferirSaida(unmount);
  });

  it('keepsSavedEditAfterSwitchingTabs', async () => {
    const corpos: { versao: string }[] = [];
    let versaoAtual = fixture.versao;
    let fluxoAtual: unknown = fixture.fluxo;
    const putComVersao: Rota = async (req) => {
      const corpo = await req.json();
      corpos.push(corpo);
      if (corpo.versao !== versaoAtual)
        return json({ codigo: 'VERSAO_ANTIGA', mensagem: CONFLITO }, 409);
      versaoAtual = `v${corpos.length + 1}`;
      fluxoAtual = corpo.fluxo;
      return json({ versao: versaoAtual, problemas: [] });
    };
    // O servidor devolve o que gravou: voltar à aba pode refazer a leitura.
    const getGravado: Rota = () =>
      json({ ...fixture, versao: versaoAtual, fluxo: fluxoAtual, problemas: [] });
    responderCom(
      rotas(construtor, {
        'GET /api/skills/agendar-mudanca/fluxo': getGravado,
        'PUT /api/skills/agendar-mudanca/fluxo': putComVersao,
      }),
    );
    const { user } = renderComProvedores(<ComTrocaDeAba />);
    await selecionar('Confirmar');
    await user.type(screen.getByRole('textbox', { name: 'Pergunta de confirmação' }), ' Primeira.');
    await vi.advanceTimersByTimeAsync(1000);
    expect(await screen.findByText(/Salvo às/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Trocar de aba' }));
    await user.click(screen.getByRole('button', { name: 'Trocar de aba' }));
    await selecionar('Confirmar');
    const campo = screen.getByRole('textbox', { name: 'Pergunta de confirmação' });
    expect(campo).toHaveValue(`${fixture.fluxo.blocos[3]?.texto ?? ''} Primeira.`);
    await user.type(campo, ' Segunda.');
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(2));
    expect(await screen.findByText(/Salvo às/)).toBeInTheDocument();
    expect(screen.queryByText('Falha ao salvar')).not.toBeInTheDocument();
    expect(corpos.map((c) => c.versao)).toEqual([fixture.versao, 'v2']);
  });

  it('keepsStructuredValueReadOnly', async () => {
    const visitantes = { campo: 'visitantes', valor: ['Ana', 'Bia'] };
    const blocos = fixture.fluxo.blocos.map((b) =>
      b.id === 'b5' && 'com' in b ? { ...b, com: [...b.com, visitantes] } : b,
    );
    const comLista = { ...fixture, fluxo: { ...fixture.fluxo, blocos } };
    const corpos: { fluxo: { blocos: { id: string; com?: unknown }[] } }[] = [];
    responderCom(
      rotas(construtor, {
        'GET /api/skills/agendar-mudanca/fluxo': () => json(comLista),
        'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda(corpos),
      }),
    );
    const { user } = montar();
    await selecionar('Gravar no sistema');
    const valores = screen.getAllByRole('textbox', { name: 'Valor' });
    expect(valores).toHaveLength(4);
    expect(valores[0]).toBeEnabled();
    expect(valores[3]).toBeDisabled();
    expect(
      screen.getByText('Este valor só pode ser editado na área Avançado.'),
    ).toBeInTheDocument();

    await user.type(screen.getByRole('textbox', { name: 'Guardar resultado em' }), '_novo');
    await vi.advanceTimersByTimeAsync(1000);
    await waitFor(() => expect(corpos).toHaveLength(1));
    const enviado = corpos[0]?.fluxo.blocos.find((b) => b.id === 'b5');
    expect(enviado?.com).toContainEqual({ campo: 'visitantes', valor: ['Ana', 'Bia'] });
  });

  it('showsFlowLevelServerProblem', async () => {
    const comAviso = {
      ...fixture,
      problemas: [
        { codigo: 'SEM_GATILHO', nivel: 'aviso', mensagem: 'Nenhuma frase leva a esta skill.' },
      ],
    };
    responderCom(
      rotas(construtor, { 'GET /api/skills/agendar-mudanca/fluxo': () => json(comAviso) }),
    );
    montar();
    expect(await screen.findByText('Nenhuma frase leva a esta skill.')).toBeInTheDocument();
  });

  it('showsToolsLoadError', async () => {
    responderCom(
      rotas(construtor, {
        'GET /api/nexus/recursos': () => json({ codigo: 'ERRO', mensagem: 'Caiu.' }, 500),
      }),
    );
    montar();
    await selecionar('Buscar dados');
    expect(await screen.findByText('Não deu para carregar as ferramentas.')).toBeInTheDocument();
    expect(screen.queryByText(/não encontrada/)).not.toBeInTheDocument();
  });

  it('showsUnknownToolWarning', async () => {
    const comAviso = {
      ...fixture,
      problemas: [
        {
          blocoId: 'b1',
          codigo: 'FERRAMENTA_DESCONHECIDA',
          nivel: 'aviso',
          mensagem: 'A ferramenta unidades.listar não existe.',
        },
      ],
    };
    responderCom(
      rotas(construtor, { 'GET /api/skills/agendar-mudanca/fluxo': () => json(comAviso) }),
    );
    montar();
    const buscar = await bloco('Buscar dados');
    expect(
      within(buscar).getByLabelText('Aviso: A ferramenta unidades.listar não existe.'),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Não salvo/)).not.toBeInTheDocument();
  });

  it('showsNoFlowMessage', async () => {
    responderCom(
      rotas(construtor, {
        'GET /api/skills/agendar-mudanca/fluxo': () =>
          json({ codigo: 'SEM_FLUXO', mensagem: 'Skill sem fluxo.' }, 404),
      }),
    );
    montar();
    expect(
      await screen.findByText('Esta skill não tem fluxo: ela responde pela base de conhecimento.'),
    ).toBeInTheDocument();
  });

  it('gestorSeesReadOnly', async () => {
    responderCom(rotas(gestor));
    const { container } = montar();
    await selecionar('Confirmar');
    expect(screen.queryByRole('button', { name: 'Adicionar bloco aqui' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Excluir bloco' })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Pergunta de confirmação' })).toBeDisabled();
    expect(container.querySelectorAll('.react-flow__node.draggable')).toHaveLength(0);
  });

  it('numbersBlocksInOrder', async () => {
    responderCom(rotas(construtor));
    montar();
    expect(within(await bloco('Buscar dados')).getByText('1')).toBeInTheDocument();
    expect(within(await bloco('Pedir escolha')).getByText('2')).toBeInTheDocument();
    expect(within(await bloco('Responder')).getByText('6')).toBeInTheDocument();
  });

  it('marksBlockTypeWithLeftBar', async () => {
    responderCom(rotas(construtor));
    montar();
    expect(await bloco('Buscar dados')).toHaveStyle({
      borderLeftColor: tokens.color.studio.category.integrate,
    });
    expect(await bloco('Confirmar')).toHaveStyle({
      borderLeftColor: tokens.color.studio.category.send,
    });
    expect(await bloco('Gravar no sistema')).toHaveStyle({
      borderLeftColor: tokens.color.studio.category.logic,
    });
  });

  it('onlyEditedBlockHasShadow', async () => {
    responderCom(rotas(construtor));
    montar();
    await selecionar('Confirmar');
    expect(await bloco('Confirmar')).toHaveStyle({ boxShadow: SOMBRA_DO_BLOCO_EM_EDICAO });
    expect(await bloco('Buscar dados')).toHaveStyle({ boxShadow: 'none' });
  });

  it('showsReadOnlyBadgeForGestor', async () => {
    responderCom(rotas(construtor));
    const { unmount } = montar();
    await bloco('Responder');
    expect(screen.queryByText('Só leitura')).not.toBeInTheDocument();
    unmount();

    responderCom(rotas(gestor));
    montar();
    expect(await screen.findByText('Só leitura')).toBeInTheDocument();
  });

  it('hidesInformationIdWhenReadOnly', async () => {
    responderCom(rotas(construtor));
    const { unmount } = montar();
    expect(await screen.findByText('data_mudanca')).toBeInTheDocument();
    unmount();

    responderCom(rotas(gestor));
    montar();
    await screen.findByDisplayValue('Data da mudança');
    expect(screen.queryByText('data_mudanca')).not.toBeInTheDocument();
  });

  it('hidesEditingControlsForGestor', async () => {
    responderCom(rotas(gestor));
    montar();
    await selecionar('Buscar dados');
    expect(screen.queryByRole('button', { name: /Adicionar bloco/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Excluir bloco' })).not.toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Como o morador vê' })).toBeInTheDocument();
  });

  it('showsReadOnlyBannerAboveCanvasForGestor', async () => {
    responderCom(rotas(construtor));
    const { unmount } = montar();
    await bloco('Responder');
    expect(screen.queryByText(/Você vê o passo a passo como gestor/)).not.toBeInTheDocument();
    unmount();

    responderCom(rotas(gestor));
    montar();
    expect(await screen.findByText(/Você vê o passo a passo como gestor/)).toBeInTheDocument();
  });

  it('showsQuestionsAsPlainTextForGestor', async () => {
    responderCom(rotas(gestor));
    montar();
    await selecionar('Perguntar');
    expect(
      screen.getByText('Para qual data você gostaria de agendar a mudança?'),
    ).toBeInTheDocument();
    expect(screen.getByText('Manhã (8h às 12h)')).toBeInTheDocument();
    expect(
      screen.queryByRole('combobox', { name: 'Informações perguntadas' }),
    ).not.toBeInTheDocument();
  });
});

describe('FluxoAba em tela estreita', () => {
  beforeAll(medirNoJsdom);
  const original = window.matchMedia;
  afterEach(() => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: original,
    });
  });

  it('narrowScreenIsReadOnly', async () => {
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: (query: string) => ({ ...original(query), matches: true }),
    });
    responderCom(rotas(construtor));
    const { container } = montar();
    await selecionar('Confirmar');
    expect(screen.queryByRole('button', { name: /Adicionar bloco/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Excluir bloco' })).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Pergunta de confirmação' })).toBeDisabled();
    expect(container.querySelectorAll('.react-flow__node.draggable')).toHaveLength(0);
  });
});
