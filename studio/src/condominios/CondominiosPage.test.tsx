import { screen } from '@testing-library/react';
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
  });

  it('removesAdministradoraAfterConfirmation', async () => {
    let removida = '';
    responderCom({
      ...rotasBase(construtor),
      'DELETE /api/admin/administradora': (req) => {
        removida = new URL(req.url).searchParams.get('id') ?? '';
        return json({ mapeamento: estadoFalso.mapeamento, administradoras: [] });
      },
    });
    const { user } = renderComProvedores(<CondominiosPage />);

    await user.click(await screen.findByRole('tab', { name: 'Administradoras' }));
    await user.click(await screen.findByRole('button', { name: 'Remover Administradora Exemplo' }));
    expect(
      screen.getByText('Remover "Administradora Exemplo"? Os condomínios dela ficam sem administradora.'),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Remover' }));

    await screen.findByText('Administradora removida.');
    expect(removida).toBe('adm-exemplo');
  });
});
