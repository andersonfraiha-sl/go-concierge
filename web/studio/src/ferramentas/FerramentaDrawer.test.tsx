import { screen, waitFor } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import type { Ferramenta } from '@/catalogo/contratos';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { FerramentaDrawer } from './FerramentaDrawer';

const ferramenta = (extra: Partial<Ferramenta> = {}): Ferramenta => ({
  name: 'reservas.criar',
  titulo: 'Criar reserva',
  kind: 'api',
  visibility: 'private',
  write: true,
  description: 'Reserva a área comum',
  params: [],
  saida: [],
  definicoes: [],
  ...extra,
});

function abrirDrawer(inicial: Ferramenta) {
  const chamadas: unknown[] = [];
  responderCom({
    'PUT /api/nexus/recurso': async (req) => {
      chamadas.push(await req.json());
      return json({ account: 'x', origem: 'instantaneo', conferido: true, recursos: [inicial] });
    },
  });
  const resultado = renderComProvedores(
    <FerramentaDrawer ferramenta={inicial} podeEditar onFechar={() => {}} />,
  );
  return { ...resultado, chamadas };
}

const abrirSecao = (user: UserEvent, nome: string) =>
  user.click(screen.getByRole('button', { name: new RegExp(`^${nome}`) }));

describe('FerramentaDrawer', () => {
  it('keepsConnectionCollapsedByDefault', () => {
    abrirDrawer(ferramenta());
    expect(screen.getByRole('button', { name: /^Conexão/ })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(screen.getByRole('button', { name: /^Básico/ })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });

  it('requiresTitle', async () => {
    const { user } = abrirDrawer(ferramenta());
    await user.clear(screen.getByLabelText('Título'));
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    expect(await screen.findByText('Informe o título.')).toBeInTheDocument();
  });

  it('rejectsInvalidCodeOnlyWhenChanged', async () => {
    const { user, chamadas } = abrirDrawer(ferramenta({ name: 'Legado_X' }));
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    await waitFor(() => expect(chamadas).toHaveLength(1));
    expect(chamadas[0]).toMatchObject({
      original: 'Legado_X',
      recurso: { name: 'Legado_X' },
    });
  });

  it('savesHeadersAsMap', async () => {
    const { user, chamadas } = abrirDrawer(ferramenta());
    await abrirSecao(user, 'Conexão');
    await user.click(screen.getByRole('button', { name: 'Adicionar cabeçalho' }));
    await user.type(screen.getByLabelText('Chave do cabeçalho'), 'Authorization');
    await user.type(screen.getByLabelText('Valor do cabeçalho'), 'Bearer x');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    await waitFor(() => expect(chamadas).toHaveLength(1));
    expect(chamadas[0]).toMatchObject({
      recurso: { conexao: { headers: { Authorization: 'Bearer x' } } },
    });
  });

  it('offersToolServerMethod', async () => {
    const { user, chamadas } = abrirDrawer(ferramenta());
    await abrirSecao(user, 'Conexão');
    await user.selectOptions(screen.getByLabelText('Método'), 'Servidor de ferramentas');
    await user.type(screen.getByLabelText('Endereço'), 'http://x');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    await waitFor(() => expect(chamadas).toHaveLength(1));
    expect(chamadas[0]).toMatchObject({ recurso: { conexao: { metodo: 'MCP' } } });
  });

  it('savesOutputFieldType', async () => {
    const { user, chamadas } = abrirDrawer(ferramenta());
    await abrirSecao(user, 'Saída');
    await user.click(screen.getByRole('button', { name: 'Adicionar campo de saída' }));
    await user.type(screen.getByLabelText('Nome do campo de saída'), 'itens');
    await user.selectOptions(screen.getByLabelText('Tipo do campo de saída'), 'array');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    await waitFor(() => expect(chamadas).toHaveLength(1));
    expect(chamadas[0]).toMatchObject({ recurso: { saida: [{ name: 'itens', type: 'array' }] } });
  });

  it('definitionOptionsOnlyForChoice', async () => {
    const { user } = abrirDrawer(ferramenta());
    await abrirSecao(user, 'Ajustes por condomínio');
    await user.click(screen.getByRole('button', { name: 'Adicionar ajuste' }));
    expect(screen.queryByLabelText('Opções do ajuste')).not.toBeInTheDocument();
    await user.selectOptions(screen.getByLabelText('Tipo do ajuste'), 'select');
    expect(screen.getByLabelText('Opções do ajuste')).toBeInTheDocument();
  });

  it('savesDefinitionTypeAndDefault', async () => {
    const { user, chamadas } = abrirDrawer(ferramenta());
    await abrirSecao(user, 'Ajustes por condomínio');
    await user.click(screen.getByRole('button', { name: 'Adicionar ajuste' }));
    await user.type(screen.getByLabelText('Nome do ajuste'), 'prazo_dias');
    await user.selectOptions(screen.getByLabelText('Tipo do ajuste'), 'number');
    await user.type(screen.getByLabelText('Padrão do ajuste'), '3');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    await waitFor(() => expect(chamadas).toHaveLength(1));
    expect(chamadas[0]).toMatchObject({
      recurso: { definicoes: [{ name: 'prazo_dias', type: 'number', default: 3 }] },
    });
  });
});
