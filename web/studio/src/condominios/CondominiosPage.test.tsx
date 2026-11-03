import { screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CondominiosPage } from './CondominiosPage';
import { construtor, estadoFalso, gestor, rotasBase } from './estadoFalso';

describe('CondominiosPage', () => {
  it('showsDefaultFirst', async () => {
    responderCom(rotasBase(construtor));
    renderComProvedores(<CondominiosPage />);

    const cartoes = await screen.findAllByRole('link');
    expect(cartoes[0]).toHaveTextContent('Padrão (todos)');
    expect(cartoes[0]).toHaveTextContent('Vale para todo condomínio que não tem decisão própria.');
    expect(cartoes[1]).toHaveTextContent('Vila Verde');
    expect(screen.getByText('Diga o que a assistente faz em cada condomínio.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Novo condomínio' })).toBeInTheDocument();
  });

  it('gestorRequestsOwnAdministradora', async () => {
    const urls: string[] = [];
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': (req) => {
        urls.push(req.url);
        return json(estadoFalso);
      },
    });
    renderComProvedores(<CondominiosPage />);

    await screen.findByText('Vila Verde');
    expect(new URL(urls[0] ?? '').searchParams.get('administradora')).toBe('adm-exemplo');
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
