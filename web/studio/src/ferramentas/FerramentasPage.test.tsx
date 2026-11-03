import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { FerramentasPage } from './FerramentasPage';

const recursos = [
  { name: 'unidades.consultar', titulo: 'Consultar unidade', kind: 'api', visibility: 'private', write: false, description: 'Busca a unidade', params: null },
  { name: 'mudancas.agendar', titulo: 'Agendar mudança', kind: 'api', visibility: 'private', write: true, description: 'Agenda a mudança', params: [{ name: 'data', type: 'string', required: true }] },
  { name: 'manual', kind: 'knowledge', visibility: 'public', write: false, description: 'Manual do condomínio', params: null },
  { name: 'mcp.externa', titulo: 'Ferramenta externa', kind: 'api', visibility: 'private', write: false, description: 'Vem de fora', origem: 'mcp', params: null },
];

const catalogo = { account: 'superlogica', origem: 'instantaneo', conferido: true, recursos };

const euComEdicao = {
  id: 'u1',
  nome: 'Ana',
  papel: 'admin',
  permissoes: ['ferramentas.editar'],
  administradoraId: null,
  dev: false,
};
const euGestor = { ...euComEdicao, papel: 'gestor', permissoes: [] };

function abrirPagina(eu = euComEdicao, extras: Parameters<typeof responderCom>[0] = {}) {
  return responderCom({
    'GET /api/eu': () => json(eu),
    'GET /api/nexus/recursos': () => json(catalogo),
    ...extras,
  });
}

describe('FerramentasPage', () => {
  it('groupsToolsByClass', async () => {
    abrirPagina();
    renderComProvedores(<FerramentasPage />);
    expect(await screen.findByRole('heading', { name: 'Consultas' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Gravações' })).toBeInTheDocument();
    expect(screen.getByRole('heading', { name: 'Conteúdos' })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Memória' })).not.toBeInTheDocument();
    expect(screen.getAllByText('Grava no sistema')).toHaveLength(1);
  });

  it('filtersByText', async () => {
    abrirPagina();
    const { user } = renderComProvedores(<FerramentasPage />);
    await screen.findByText('Agendar mudança');
    await user.type(screen.getByLabelText('Filtrar'), 'mudança');
    expect(screen.getByText('Agendar mudança')).toBeInTheDocument();
    expect(screen.queryByText('Consultar unidade')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Conteúdos' })).not.toBeInTheDocument();
  });

  it('savesTool', async () => {
    let corpo: unknown = null;
    abrirPagina(euComEdicao, {
      'PUT /api/nexus/recurso': async (req) => {
        corpo = await req.json();
        return json(catalogo);
      },
    });
    const { user } = renderComProvedores(<FerramentasPage />);
    await user.click(await screen.findByText('Agendar mudança'));
    const descricao = await screen.findByLabelText('Descrição');
    await user.clear(descricao);
    await user.type(descricao, 'Agenda e confirma');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    expect(await screen.findByText('Ferramenta salva.')).toBeInTheDocument();
    expect(corpo).toMatchObject({
      original: 'mudancas.agendar',
      recurso: { name: 'mudancas.agendar', write: true, description: 'Agenda e confirma' },
    });
  });

  it('externalToolIsReadOnly', async () => {
    abrirPagina();
    const { user } = renderComProvedores(<FerramentasPage />);
    await user.click(await screen.findByText('Ferramenta externa'));
    expect(await screen.findByText('Esta ferramenta é mantida fora do Studio.')).toBeInTheDocument();
    expect(screen.getByLabelText('Nome')).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Salvar' })).toBeDisabled();
  });

  it('marksToolAsExternalWhenServerRefuses', async () => {
    abrirPagina(euComEdicao, {
      'PUT /api/nexus/recurso': () => json({ codigo: 'CATALOGO_AO_VIVO', mensagem: 'edite lá' }, 409),
    });
    const { user } = renderComProvedores(<FerramentasPage />);
    await user.click(await screen.findByText('Consultar unidade'));
    await user.click(await screen.findByRole('button', { name: 'Salvar' }));
    expect(await screen.findByText('Esta ferramenta é mantida fora do Studio.')).toBeInTheDocument();
    expect(screen.getByLabelText('Nome')).toBeDisabled();
  });

  it('gestorCannotEdit', async () => {
    abrirPagina(euGestor);
    const { user } = renderComProvedores(<FerramentasPage />);
    await user.click(await screen.findByText('Agendar mudança'));
    expect(screen.queryByRole('button', { name: 'Nova ferramenta' })).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Descrição')).not.toBeInTheDocument();
  });

  it('rejectsInvalidSampleJson', async () => {
    let gravou = false;
    abrirPagina(euComEdicao, {
      'PUT /api/nexus/recurso': () => {
        gravou = true;
        return json(catalogo);
      },
    });
    const { user } = renderComProvedores(<FerramentasPage />);
    await user.click(await screen.findByText('Agendar mudança'));
    const amostra = await screen.findByLabelText('Amostra de retorno');
    await user.click(amostra);
    await user.paste('{x');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    expect(await screen.findByText('JSON inválido')).toBeInTheDocument();
    expect(gravou).toBe(false);
  });

  it('removesToolAfterConfirmation', async () => {
    let consulta = '';
    abrirPagina(euComEdicao, {
      'DELETE /api/nexus/recurso': (req) => {
        consulta = new URL(req.url).searchParams.get('nome') ?? '';
        return json(catalogo);
      },
    });
    const { user } = renderComProvedores(<FerramentasPage />);
    await user.click(await screen.findByText('Agendar mudança'));
    await user.click(await screen.findByRole('button', { name: 'Remover' }));
    const confirmacao = await screen.findByRole('dialog', { name: 'Remover ferramenta' });
    expect(
      within(confirmacao).getByText('Remover a ferramenta "Agendar mudança"? As skills que usam deixam de achá-la.'),
    ).toBeInTheDocument();
    await user.click(within(confirmacao).getByRole('button', { name: 'Remover' }));
    await waitFor(() => expect(consulta).toBe('mudancas.agendar'));
    expect(await screen.findByText('Ferramenta removida.')).toBeInTheDocument();
  });

  it('showsRetryOnLoadError', async () => {
    responderCom({
      'GET /api/eu': () => json(euComEdicao),
      'GET /api/nexus/recursos': () => json({ codigo: 'X', mensagem: 'caiu' }, 500),
    });
    renderComProvedores(<FerramentasPage />);
    expect(await screen.findByRole('button', { name: 'Tentar de novo' })).toBeInTheDocument();
  });
});
