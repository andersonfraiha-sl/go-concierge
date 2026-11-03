import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeAll, describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import fixture from '@/fluxo/modelo/agendar-mudanca.fixture.json';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
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
    { name: 'unidades.listar', titulo: 'Listar unidades', kind: 'api', visibility: 'private', write: false, description: '' },
    { name: 'mudancas.agendar', titulo: 'Agendar mudança', kind: 'api', visibility: 'private', write: true, description: '' },
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
function putQueGuarda(corpos: unknown[], resposta: () => Response = () => json({ versao: 'v2', problemas: [] })): Rota {
  return async (req) => {
    corpos.push(await req.json());
    return resposta();
  };
}

const montar = () => renderComProvedores(<FluxoAba nome="agendar-mudanca" caminho="skills/agendar-mudanca" />);

const rotulosDosBlocos = (): (string | null)[] =>
  within(screen.getByRole('region', { name: 'Fluxo' }))
    .getAllByRole('article')
    .map((bloco) => bloco.getAttribute('aria-label'));

const bloco = (rotulo: string): Promise<HTMLElement> => screen.findByRole('article', { name: rotulo });

/** Clique sem mousedown: o d3-drag do nó arrastável lê `event.view`, que o user-event deixa nulo no jsdom. */
const selecionar = async (rotulo: string): Promise<void> => {
  fireEvent.click(await bloco(rotulo));
};

const esperarUmSegundoESobra = () => new Promise((resolve) => setTimeout(resolve, 1300));

/** O React Flow só mede os nós no callback do ResizeObserver; sem medida, nem nós visíveis nem arestas. */
function medirNoJsdom(): void {
  class ResizeObserverQueMede {
    constructor(private readonly callback: ResizeObserverCallback) {}
    observe(target: Element): void {
      const contentRect = { x: 0, y: 0, top: 0, left: 0, width: 800, height: 600, bottom: 600, right: 800 };
      // Depois do commit: o React Flow só guarda o DOM do canvas no efeito do pai, que roda após o dos nós.
      setTimeout(() => {
        if (target.isConnected) this.callback([{ target, contentRect } as ResizeObserverEntry], this as unknown as ResizeObserver);
      }, 0);
    }
    unobserve(): void {}
    disconnect(): void {}
  }
  for (const alvo of [globalThis, window]) {
    Object.defineProperty(alvo, 'ResizeObserver', { value: ResizeObserverQueMede, configurable: true });
  }
  Object.defineProperties(HTMLElement.prototype, {
    offsetHeight: { configurable: true, get: () => 600 },
    offsetWidth: { configurable: true, get: () => 800 },
  });
}

describe('FluxoAba', () => {
  beforeAll(medirNoJsdom);

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
    await waitFor(() => expect(container.querySelectorAll('.react-flow__node.draggable')).toHaveLength(5));
  });

  it('insertsBlockFromPlusBetweenBlocks', async () => {
    responderCom(rotas(construtor));
    const { user } = montar();
    await bloco('Responder');
    const [entrePrimeiroESegundo] = await screen.findAllByRole('button', { name: 'Adicionar bloco aqui' });
    if (!entrePrimeiroESegundo) throw new Error('sem + entre blocos');
    await user.click(entrePrimeiroESegundo);
    await user.click(screen.getByRole('menuitem', { name: /^Confirmar/ }));
    await waitFor(() => expect(rotulosDosBlocos()).toHaveLength(7));
    expect(rotulosDosBlocos()[1]).toBe('Confirmar');
    const painel = screen.getByRole('region', { name: 'Bloco selecionado' });
    expect(within(painel).getByRole('heading', { name: 'Confirmar' })).toBeInTheDocument();
    expect(within(painel).getByRole('textbox', { name: 'Pergunta de confirmação' })).toHaveValue('');
  });

  it('marksGravarWithoutConfirmar', async () => {
    const corpos: unknown[] = [];
    responderCom(rotas(construtor, { 'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda(corpos) }));
    const { user } = montar();
    await selecionar('Confirmar');
    await user.click(screen.getByRole('button', { name: 'Excluir bloco' }));
    const dialogo = screen.getByRole('dialog', { name: 'Excluir o bloco "Confirmar"?' });
    await user.click(within(dialogo).getByRole('button', { name: 'Excluir' }));

    expect(await screen.findByLabelText('Erro: Coloque um Confirmar antes de gravar no sistema.')).toBeInTheDocument();
    expect(screen.getByText('Não salvo: corrija o bloco marcado')).toBeInTheDocument();
    await selecionar('Gravar no sistema');
    expect(screen.getByText('Coloque um Confirmar antes de gravar no sistema.')).toBeVisible();
    await esperarUmSegundoESobra();
    expect(corpos).toEqual([]);
  });

  it('savesOneSecondAfterEdit', async () => {
    const corpos: unknown[] = [];
    responderCom(rotas(construtor, { 'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda(corpos) }));
    const { user } = montar();
    await selecionar('Confirmar');
    const campo = screen.getByRole('textbox', { name: 'Pergunta de confirmação' });
    await user.clear(campo);
    await user.type(campo, 'Posso agendar?');
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos[0]).toMatchObject({
      versao: fixture.versao,
      fluxo: { blocos: expect.arrayContaining([expect.objectContaining({ id: 'b4', texto: 'Posso agendar?' })]) },
    });
    expect(await screen.findByText(/Salvo às/)).toBeInTheDocument();
  });

  it('showsServerRejection', async () => {
    const rejeicao = () =>
      json(
        {
          codigo: 'FLUXO_INVALIDO',
          mensagem: 'O fluxo tem problemas.',
          problemas: [{ blocoId: 'b5', codigo: 'RECURSO_RECUSADO', nivel: 'erro', mensagem: 'A ferramenta recusou os dados.' }],
        },
        422,
      );
    responderCom(rotas(construtor, { 'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda([], rejeicao) }));
    const { user } = montar();
    await selecionar('Confirmar');
    await user.type(screen.getByRole('textbox', { name: 'Pergunta de confirmação' }), '!');
    expect(await screen.findByText('Falha ao salvar', {}, { timeout: 3000 })).toBeInTheDocument();
    expect(
      within(await bloco('Gravar no sistema')).getByLabelText('Erro: A ferramenta recusou os dados.'),
    ).toBeInTheDocument();
  });

  it('showsConflictAndKeepsEdit', async () => {
    const conflito = () => json({ codigo: 'VERSAO_ANTIGA', mensagem: CONFLITO }, 409);
    responderCom(rotas(construtor, { 'PUT /api/skills/agendar-mudanca/fluxo': putQueGuarda([], conflito) }));
    const { user } = montar();
    await selecionar('Confirmar');
    const campo = screen.getByRole('textbox', { name: 'Pergunta de confirmação' });
    await user.clear(campo);
    await user.type(campo, 'Posso agendar?');
    expect(await screen.findByText('Falha ao salvar', {}, { timeout: 3000 })).toBeInTheDocument();
    expect(screen.getByLabelText(CONFLITO)).toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Pergunta de confirmação' })).toHaveValue('Posso agendar?');
  });

  it('showsUnknownToolWarning', async () => {
    const comAviso = {
      ...fixture,
      problemas: [
        { blocoId: 'b1', codigo: 'FERRAMENTA_DESCONHECIDA', nivel: 'aviso', mensagem: 'A ferramenta unidades.listar não existe.' },
      ],
    };
    responderCom(rotas(construtor, { 'GET /api/skills/agendar-mudanca/fluxo': () => json(comAviso) }));
    montar();
    const buscar = await bloco('Buscar dados');
    expect(within(buscar).getByLabelText('Aviso: A ferramenta unidades.listar não existe.')).toBeInTheDocument();
    expect(screen.queryByText(/Não salvo/)).not.toBeInTheDocument();
  });

  it('showsNoFlowMessage', async () => {
    responderCom(
      rotas(construtor, {
        'GET /api/skills/agendar-mudanca/fluxo': () => json({ codigo: 'SEM_FLUXO', mensagem: 'Skill sem fluxo.' }, 404),
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
});

describe('FluxoAba em tela estreita', () => {
  beforeAll(medirNoJsdom);
  const original = window.matchMedia;
  afterEach(() => {
    Object.defineProperty(window, 'matchMedia', { configurable: true, writable: true, value: original });
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
