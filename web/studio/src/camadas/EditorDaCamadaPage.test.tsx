import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Eu } from '@/acesso/eu';
import fixture from '@/fluxo/modelo/agendar-mudanca.fixture.json';
import { json, responderCom } from '@/test/fetchFalso';
import { medirNoJsdom } from '@/test/medirNoJsdom';
import { renderComProvedores } from '@/test/render';
import { EditorDaCamadaPage } from './EditorDaCamadaPage';

const gestor: Eu = {
  id: 'u1',
  nome: 'Bia',
  papel: 'gestor_administradora',
  permissoes: ['skills.ler', 'skills.editar_textos', 'skills.editar_etapas'],
  administradoraId: 'adm-1',
  dev: false,
};
const soTextos: Eu = { ...gestor, permissoes: ['skills.ler', 'skills.editar_textos'] };

const recursos = ['unidades.listar', 'mudancas.agendar'].map((name) => ({
  name,
  titulo: name === 'unidades.listar' ? 'Listar unidades' : 'Agendar mudança',
  kind: 'api',
  visibility: 'private',
  write: name === 'mudancas.agendar',
  description: '',
}));

const camadaLida = {
  camada: { skill: 'agendar-mudanca', tipo: 'administradora', id: 'adm-1' },
  documento: {
    textos: {
      titulo: 'Agendar mudança',
      quandoUsar: 'Quando o morador quer marcar a mudança.',
      frasesDeExemplo: ['quero agendar minha mudança'],
      palavrasChave: ['mudança'],
      temFluxo: true,
      resumo: 'Agenda o dia da mudança.',
      visibilidade: 'private',
    },
    fluxo: fixture.fluxo,
    ferramentas: ['unidades.listar'],
    orientacao: '',
  },
  versao: 'v1',
  temRascunho: true,
  publicada: { numero: 2, autor: 'Ana', criadoEm: '2026-11-04T10:00:00Z' },
  ferramentasDaBase: ['unidades.listar'],
  problemas: [],
};

const BASE = '/api/skills/agendar-mudanca';
const DA_ADMINISTRADORA = `${BASE}/administradoras/adm-1`;
const DO_CONDOMINIO = `${BASE}/condominios/vila-verde`;
const camadaDoCondominio = {
  ...camadaLida,
  camada: { ...camadaLida.camada, tipo: 'condominio', id: 'vila-verde' },
};

type Rota = (req: Request) => Response | Promise<Response>;

function rotas(eu: Eu, extras: Record<string, Rota> = {}): Record<string, Rota> {
  return {
    'GET /api/eu': () => json(eu),
    'GET /api/admin/estado': () => json({ mapeamento: {}, administradoras: [], casos: [] }),
    'GET /api/atualizacoes': () => json({ atualizacoes: [], avisos: [] }),
    [`GET ${DA_ADMINISTRADORA}`]: () => json(camadaLida),
    [`GET ${DO_CONDOMINIO}`]: () => json(camadaDoCondominio),
    'GET /api/nexus/recursos': () =>
      json({ account: 'conta', origem: 'nexus', conferido: true, recursos }),
    ...extras,
  };
}

/** Guarda o corpo recebido e responde com a próxima versão. */
function putQueGuarda(
  corpos: unknown[],
  resposta: () => Response = () => json({ versao: 'v2', problemas: [] }),
): Rota {
  return async (req) => {
    corpos.push(await req.json());
    return resposta();
  };
}

type Montagem = ReturnType<typeof montar>;

const montar = (destino = 'administradoras/adm-1') =>
  renderComProvedores(<EditorDaCamadaPage />, {
    rota: `/skills/agendar-mudanca/${destino}`,
    caminho: '/skills/:nome/:tipo/:id',
  });

const esperarUmSegundoESobra = () => vi.advanceTimersByTimeAsync(1300);

async function editarQuandoUsar(user: Montagem['user']): Promise<void> {
  const campo = await screen.findByRole('textbox', { name: 'Como reconhecer o pedido' });
  await user.type(campo, ' Também por telefone.');
}

async function abrirFluxo(user: Montagem['user']): Promise<void> {
  await user.click(await screen.findByRole('tab', { name: 'Fluxo' }));
}

describe('EditorDaCamadaPage', () => {
  beforeAll(medirNoJsdom);
  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('autosavesDraftOfTheLayer', async () => {
    const corpos: unknown[] = [];
    responderCom(rotas(gestor, { [`PUT ${DA_ADMINISTRADORA}/rascunho`]: putQueGuarda(corpos) }));
    const { user } = montar();
    expect(await screen.findByText('Personalizar · Agendar mudança')).toBeInTheDocument();
    await editarQuandoUsar(user);
    await esperarUmSegundoESobra();

    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toMatchObject({
      versao: 'v1',
      documento: {
        textos: { quandoUsar: 'Quando o morador quer marcar a mudança. Também por telefone.' },
        ferramentas: ['unidades.listar'],
      },
    });
    expect(await screen.findByText(/Salvo às/)).toBeInTheDocument();
  });

  it('disablesPublishWhileSaving', async () => {
    let concluir: (() => void) | undefined;
    const lento: Rota = () =>
      new Promise<Response>((resolver) => {
        concluir = () => resolver(json({ versao: 'v2', problemas: [] }));
      });
    responderCom(rotas(gestor, { [`PUT ${DA_ADMINISTRADORA}/rascunho`]: lento }));
    const { user } = montar();
    await editarQuandoUsar(user);
    await esperarUmSegundoESobra();

    expect(await screen.findByText('Salvando…')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Publicar' })).toBeDisabled();
    concluir?.();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Publicar' })).toBeEnabled());
  });

  it('publishesWithSummary', async () => {
    const publicacoes: unknown[] = [];
    responderCom(
      rotas(gestor, {
        [`POST ${DA_ADMINISTRADORA}/publicar`]: async (req) => {
          publicacoes.push(await req.json());
          return json({ numero: 3 }, 201);
        },
      }),
    );
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Publicar' }));
    const dialogo = await screen.findByRole('dialog', {
      name: 'Publicar sua versão',
    });
    await user.type(within(dialogo).getByRole('textbox', { name: 'Resumo' }), 'Ajustei o horário');
    await user.click(within(dialogo).getByRole('button', { name: 'Publicar' }));

    await waitFor(() => expect(publicacoes).toEqual([{ resumo: 'Ajustei o horário' }]));
    expect(await screen.findByText('Versão publicada.')).toBeInTheDocument();
  });

  it('showsPublishRefusalInTheDialog', async () => {
    responderCom(
      rotas(gestor, {
        [`POST ${DA_ADMINISTRADORA}/publicar`]: () =>
          json({ codigo: 'NADA_A_PUBLICAR', mensagem: 'Não há mudanças para publicar.' }, 409),
      }),
    );
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Publicar' }));
    const dialogo = await screen.findByRole('dialog');
    await user.click(within(dialogo).getByRole('button', { name: 'Publicar' }));

    expect(await within(dialogo).findByText('Não há mudanças para publicar.')).toBeInTheDocument();
  });

  it('offersOnlyBaseToolsInFlow', async () => {
    responderCom(rotas(gestor));
    const { user } = montar();
    await abrirFluxo(user);
    fireEvent.click(await screen.findByRole('article', { name: 'Buscar dados' }));
    await user.click(await screen.findByRole('combobox', { name: 'Ferramenta' }));

    const lista = within(await screen.findByRole('listbox'));
    expect(lista.getByRole('option', { name: 'Listar unidades' })).toBeInTheDocument();
    expect(lista.queryByRole('option', { name: 'Agendar mudança' })).not.toBeInTheDocument();
  });

  it('hidesOrientationTabForAdministradora', async () => {
    responderCom(rotas(gestor));
    montar();
    await screen.findByRole('tab', { name: 'Fluxo' });
    expect(screen.queryByRole('tab', { name: 'Orientação' })).not.toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Histórico' })).toBeInTheDocument();
  });

  it('editsOrientationOfTheCondominio', async () => {
    const corpos: unknown[] = [];
    responderCom(rotas(gestor, { [`PUT ${DO_CONDOMINIO}/rascunho`]: putQueGuarda(corpos) }));
    const { user } = montar('condominios/vila-verde');
    await user.click(await screen.findByRole('tab', { name: 'Orientação' }));
    await user.type(
      screen.getByRole('textbox', { name: 'Orientação para este condomínio' }),
      'Mudanças só de manhã',
    );
    await esperarUmSegundoESobra();

    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toMatchObject({ documento: { orientacao: 'Mudanças só de manhã' } });
  });

  it('showsReadOnlyFlowWithoutEditStepsPermission', async () => {
    responderCom(rotas(soTextos));
    const { user } = montar();
    await abrirFluxo(user);

    expect(await screen.findByText('Só leitura')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Adicionar bloco aqui' })).not.toBeInTheDocument();
  });

  it('showsToolErrorOnTheBlock', async () => {
    const recusa = () =>
      json(
        {
          codigo: 'FERRAMENTA_FORA_DA_BASE',
          mensagem: 'Há ferramenta que a base não usa.',
          problemas: [
            {
              blocoId: 'b5',
              codigo: 'FERRAMENTA_FORA_DA_BASE',
              nivel: 'erro',
              mensagem: 'A base não usa esta ferramenta.',
            },
          ],
        },
        422,
      );
    responderCom(
      rotas(gestor, { [`PUT ${DA_ADMINISTRADORA}/rascunho`]: putQueGuarda([], recusa) }),
    );
    // Ao sair da tela com a gravação falhada o autosave tenta de novo e registra o erro: é esperado.
    const erros = vi.spyOn(console, 'error').mockImplementation(() => {});
    const { user, unmount } = montar();
    await editarQuandoUsar(user);
    await esperarUmSegundoESobra();
    expect(await screen.findByText('Falha ao salvar')).toBeInTheDocument();

    await abrirFluxo(user);
    expect(
      within(await screen.findByRole('article', { name: 'Gravar no sistema' })).getByLabelText(
        'Erro: A base não usa esta ferramenta.',
      ),
    ).toBeInTheDocument();
    unmount();
    erros.mockRestore();
  });

  it('showsMessageForUnknownLayerRoute', async () => {
    responderCom(rotas(gestor));
    renderComProvedores(<EditorDaCamadaPage />, {
      rota: '/skills/agendar-mudanca/blocos/x',
      caminho: '/skills/:nome/:tipo/:id',
    });
    expect(await screen.findByText('Não encontramos esta versão.')).toBeInTheDocument();
  });

  it('showsUpdateBannerAboveTheTabs', async () => {
    const pendente = {
      id: 'at-1',
      camada: camadaLida.camada,
      estado: 'pendente',
      para: { numero: 3, resumo: '' },
      prazoEm: null,
      trechos: 2,
      conflitos: 0,
    };
    responderCom(
      rotas(gestor, {
        'GET /api/atualizacoes': () => json({ atualizacoes: [pendente], avisos: [] }),
      }),
    );
    montar();
    expect(await screen.findByText('Essa skill tem atualizações')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Revisar' })).toHaveAttribute(
      'href',
      '/atualizacoes/at-1',
    );
  });

  it('reloadsTheEditorAfterRestoringAVersion', async () => {
    let tituloNoServidor = 'Agendar mudança';
    const versao = (numero: number) => ({
      numero,
      origem: 'publicacao',
      autor: 'Ana',
      resumo: '',
      motivo: '',
      criadoEm: `2026-11-0${numero}T12:00:00Z`,
      decisao: null,
    });
    responderCom(
      rotas(gestor, {
        [`GET ${DA_ADMINISTRADORA}`]: () =>
          json({
            ...camadaLida,
            documento: {
              ...camadaLida.documento,
              textos: { ...camadaLida.documento.textos, titulo: tituloNoServidor },
            },
          }),
        [`GET ${DA_ADMINISTRADORA}/historico`]: () => json({ versoes: [versao(2), versao(1)] }),
        [`POST ${DA_ADMINISTRADORA}/voltar`]: () => {
          tituloNoServidor = 'Agendar mudança antiga';
          return json({ numero: 3 }, 201);
        },
      }),
    );
    const { user } = montar();
    await user.click(await screen.findByRole('tab', { name: 'Histórico' }));
    const antiga = within(await screen.findByRole('listitem', { name: 'Versão 1' }));
    await user.click(await antiga.findByRole('button', { name: 'Voltar para esta versão' }));
    await user.click(screen.getByRole('button', { name: 'Voltar' }));

    expect(await screen.findByText('Personalizar · Agendar mudança antiga')).toBeInTheDocument();
  });
});
