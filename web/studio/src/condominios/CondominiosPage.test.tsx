import { screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CondominiosPage } from './CondominiosPage';
import { construtor, estadoFalso, gestor, rotasBase } from './estadoFalso';

/** Vila Verde some; entram um condomínio sem ids de integração e um sem administradora. */
const estadoComVarios = structuredClone(estadoFalso);
estadoComVarios.mapeamento.condominios = [
  {
    id: 'vila-verde',
    nome: 'Condomínio Vila Verde',
    administradora_id: 'adm-exemplo',
    account: 'acc-1',
    company_id: 'gruvi-1',
    tenant: 'atende-1',
  },
  {
    id: 'acores',
    nome: 'Residencial Açores',
    administradora_id: 'adm-exemplo',
    account: 'acc-2',
    company_id: 'gruvi-2',
    tenant: '',
  },
  {
    id: 'solar',
    nome: 'Edifício Solar',
    administradora_id: '',
    account: 'acc-3',
    company_id: 'gruvi-3',
    tenant: 'atende-3',
  },
  {
    id: 'sem-nada',
    nome: 'Condomínio Sem Nada',
    administradora_id: '',
    account: 'acc-4',
    company_id: 'gruvi-4',
    tenant: '',
  },
];

const renderComVarios = (eu = construtor) => {
  responderCom({ ...rotasBase(eu), 'GET /api/admin/estado': () => json(estadoComVarios) });
  return renderComProvedores(<CondominiosPage />);
};

describe('CondominiosPage', () => {
  it('listsCondominiosAsTableWithSkillCount', async () => {
    renderComVarios();

    const linha = (await screen.findByText('Condomínio Vila Verde')).closest('tr');
    expect(linha).not.toBeNull();
    expect(within(linha as HTMLElement).getByText('1 de 2')).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Skills ativas' })).toBeInTheDocument();
    expect(screen.getByText('Diga o que o concierge faz em cada condomínio.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Novo condomínio' })).toBeInTheDocument();
  });

  it('pinsDefaultFirst', async () => {
    renderComVarios();
    await screen.findByText('Condomínio Vila Verde');

    const linhas = screen.getAllByRole('row');
    expect(linhas[1]).toHaveTextContent('Padrão (todos)');
    expect(linhas[1]).toHaveTextContent('Vale para todo condomínio que não tem decisão própria.');
    expect(within(linhas[1] as HTMLElement).getByText('Base')).toBeInTheDocument();
  });

  it('showsStatusColumnWithReadyBadge', async () => {
    renderComVarios();

    expect(await screen.findByRole('columnheader', { name: 'Situação' })).toBeInTheDocument();
    const linha = (await screen.findByText('Condomínio Vila Verde')).closest('tr');
    expect(within(linha as HTMLElement).getByText('Pronto')).toBeInTheDocument();
  });

  it('filtersBySearch', async () => {
    const { user } = renderComVarios();
    await screen.findByText('Condomínio Vila Verde');

    await user.type(screen.getByRole('textbox', { name: 'Buscar condomínio' }), 'acores');

    expect(screen.getByText('Residencial Açores')).toBeInTheDocument();
    expect(screen.queryByText('Condomínio Vila Verde')).not.toBeInTheDocument();
    expect(screen.queryByText('Edifício Solar')).not.toBeInTheDocument();
    expect(screen.getByText('Padrão (todos)')).toBeInTheDocument();
  });

  it('filtersByAdministradoraFromQuery', async () => {
    responderCom(rotasBase(construtor));
    const { user } = renderComProvedores(<CondominiosPage />, {
      rota: '/condominios?administradora=adm-exemplo',
    });

    const filtro = (await screen.findByRole('combobox', {
      name: 'Administradora',
    })) as HTMLSelectElement;
    expect(filtro.value).toBe('adm-exemplo');

    await user.selectOptions(filtro, 'Todas as administradoras');

    expect(filtro.value).toBe('');
  });

  it('flagsMissingIntegrationIds', async () => {
    renderComVarios();

    const linha = (await screen.findByText('Residencial Açores')).closest('tr');
    expect(within(linha as HTMLElement).getByText('Faltam ids')).toBeInTheDocument();
  });

  it('highlightsWithoutAdministradora', async () => {
    renderComVarios();

    const linha = (await screen.findByText('Edifício Solar')).closest('tr');
    expect(within(linha as HTMLElement).getByText('Sem administradora')).toBeInTheDocument();
  });

  it('prioritizesMissingIdsOverMissingAdministradora', async () => {
    renderComVarios();

    const linha = (await screen.findByText('Condomínio Sem Nada')).closest('tr');
    expect(within(linha as HTMLElement).getByText('Faltam ids')).toBeInTheDocument();
    expect(within(linha as HTMLElement).queryByText('Sem administradora')).not.toBeInTheDocument();
  });

  it('hidesAdministradoraColumnForGestor', async () => {
    renderComVarios(gestor);
    await screen.findByText('Condomínio Vila Verde');

    expect(screen.queryByRole('columnheader', { name: 'Administradora' })).not.toBeInTheDocument();
  });

  it('opensTabFromQuery', async () => {
    responderCom(rotasBase(construtor));
    renderComProvedores(<CondominiosPage />, { rota: '/condominios?aba=administradoras' });

    expect(
      await screen.findByRole('textbox', { name: 'Nome da administradora' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Nenhum condomínio ainda.')).not.toBeInTheDocument();
  });

  it('opensMatrixTabFromQuery', async () => {
    responderCom(rotasBase(construtor));
    renderComProvedores(<CondominiosPage />, { rota: '/condominios?aba=skills' });

    expect(await screen.findByRole('columnheader', { name: 'Vila Verde' })).toBeInTheDocument();
    expect(screen.queryByText('Nenhum condomínio ainda.')).not.toBeInTheDocument();
  });

  it('hidesAdministradorasWithoutPermission', async () => {
    responderCom(rotasBase(gestor));
    renderComProvedores(<CondominiosPage />);

    await screen.findByText('Vila Verde');
    expect(screen.queryByRole('tab', { name: 'Administradoras' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Novo condomínio' })).not.toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Tela antiga' })).not.toBeInTheDocument();
  });

  it('linksLegacyScreensForSuperlogica', async () => {
    responderCom(rotasBase(construtor));
    renderComProvedores(<CondominiosPage />);

    const telas = await screen.findByRole('navigation', { name: 'Tela antiga' });
    const destinos = within(telas)
      .getAllByRole('link')
      .map((link) => link.getAttribute('href'));
    expect(destinos).toEqual(['/admin', '/backdoor/administradoras', '/backdoor/payloads']);
  });

  it('removesAdministradoraAfterConfirmation', async () => {
    let removida = '';
    const comVazia = structuredClone(estadoFalso);
    comVazia.administradoras.push({ id: 'adm-vazia', nome: 'Administradora Vazia' });
    responderCom({
      ...rotasBase(construtor),
      'GET /api/admin/estado': () => json(comVazia),
      'DELETE /api/admin/administradora': (req) => {
        removida = new URL(req.url).searchParams.get('id') ?? '';
        return json({
          mapeamento: estadoFalso.mapeamento,
          administradoras: estadoFalso.administradoras,
        });
      },
    });
    const { user } = renderComProvedores(<CondominiosPage />);

    await user.click(await screen.findByRole('tab', { name: 'Administradoras' }));
    await user.click(await screen.findByRole('button', { name: 'Remover Administradora Vazia' }));
    expect(
      screen.getByText('Remover "Administradora Vazia"? Não dá para desfazer.'),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Remover' }));

    await screen.findByText('Administradora removida.');
    expect(removida).toBe('adm-vazia');
  });

  it('blocksRemovingAdministradoraWithCondominios', async () => {
    responderCom(rotasBase(construtor));
    const { user } = renderComProvedores(<CondominiosPage />);

    await user.click(await screen.findByRole('tab', { name: 'Administradoras' }));

    expect(
      await screen.findByRole('button', { name: 'Remover Administradora Exemplo' }),
    ).toBeDisabled();
    expect(screen.getByText('Mova os condomínios dela antes de remover.')).toBeInTheDocument();
  });

  it('rejectsDuplicateCondominioName', async () => {
    responderCom(rotasBase(construtor));
    const { user } = renderComProvedores(<CondominiosPage />);

    await user.click(await screen.findByRole('button', { name: 'Novo condomínio' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Vila Verde');
    await user.click(screen.getByRole('button', { name: 'Criar condomínio' }));

    expect(await screen.findByText('Já existe um condomínio com esse nome.')).toBeInTheDocument();
  });

  it('offersRetryWhenLoadFails', async () => {
    let chamadas = 0;
    responderCom({
      ...rotasBase(construtor),
      'GET /api/admin/estado': () => {
        chamadas += 1;
        return chamadas === 1 ? json({ codigo: 'ERRO', mensagem: 'caiu' }, 500) : json(estadoFalso);
      },
    });
    const { user } = renderComProvedores(<CondominiosPage />);

    await user.click(await screen.findByRole('button', { name: 'Tentar de novo' }));

    expect(await screen.findByText('Vila Verde')).toBeInTheDocument();
  });
});
