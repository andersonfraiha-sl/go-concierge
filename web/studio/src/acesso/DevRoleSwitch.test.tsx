import { fireEvent, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { StudioLayout } from '@/app/StudioLayout';
import { CHAVE_USUARIO_DEV } from '@/shared/api/http';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import type { Eu } from './eu';

const base: Eu = {
  id: 'admin-superlogica',
  nome: 'Ana',
  papel: 'admin_superlogica',
  permissoes: ['skills.ler', 'condominios.editar', 'studio.avancado'],
  administradoraId: null,
  dev: true,
};

describe('acesso na barra lateral e no topo', () => {
  it('hidesAdvancedForGestor', async () => {
    const gestor: Eu = {
      ...base,
      papel: 'gestor_administradora',
      permissoes: ['skills.ler'],
      administradoraId: 'adm-1',
    };
    responderCom({ 'GET /api/eu': () => json(gestor) });

    renderComProvedores(<StudioLayout />);

    expect(await screen.findByRole('link', { name: 'Skills' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Avançado' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Condomínios' })).not.toBeInTheDocument();
  });

  it('showsAdvancedAndUserNameWithPermission', async () => {
    responderCom({ 'GET /api/eu': () => json(base) });

    renderComProvedores(<StudioLayout />);

    expect(await screen.findByRole('link', { name: 'Avançado' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Condomínios' })).toBeInTheDocument();
    expect(screen.getByText('Ana')).toBeInTheDocument();
  });

  it('switchesDevUserAndSendsHeader', async () => {
    const falso = responderCom({ 'GET /api/eu': () => json(base) });
    const { user } = renderComProvedores(<StudioLayout />);
    await screen.findByRole('combobox', { name: 'Ver como' });
    const antes = falso.mock.calls.length;

    await user.click(screen.getByRole('combobox', { name: 'Ver como' }));
    await user.click(within(screen.getByRole('listbox')).getByText('Administradora (gestor)'));

    expect(localStorage.getItem(CHAVE_USUARIO_DEV)).toBe('gestor-exemplo');
    await vi.waitFor(() => expect(falso.mock.calls.length).toBeGreaterThan(antes));
    const ultima = new Headers(falso.mock.calls.at(-1)?.[1]?.headers);
    expect(ultima.get('X-Dev-Usuario')).toBe('gestor-exemplo');
    expect(ultima.get('X-Studio-Papel')).toBeNull();
  });

  it('showsRoleFromServerNotFromStorage', async () => {
    localStorage.setItem(CHAVE_USUARIO_DEV, 'admin-superlogica');
    responderCom({ 'GET /api/eu': () => json({ ...base, id: 'gestor-avancado' }) });

    renderComProvedores(<StudioLayout />);

    expect(await screen.findByRole('combobox', { name: 'Ver como' })).toHaveTextContent(
      'Administradora com Avançado',
    );
  });

  it('showsRetryInsteadOfNoAccessWhenEuFails', async () => {
    responderCom({ 'GET /api/eu': () => json({ codigo: 'X', mensagem: 'falhou' }, 500) });

    const { user } = renderComProvedores(<StudioLayout />);

    expect(await screen.findByText('Não foi possível carregar seu acesso.')).toBeInTheDocument();
    expect(screen.queryByText('Você não tem acesso a esta área.')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Tentar de novo' })).toBeEnabled();
    await user.click(screen.getByRole('button', { name: 'Tentar de novo' }));
  });

  it('showsOnlyUnrecognizedScreenWithWayBackInDev', async () => {
    localStorage.setItem(CHAVE_USUARIO_DEV, 'fantasma');
    localStorage.setItem('studio.copilotoAberto', 'true');
    responderCom({
      'GET /api/eu': (req) =>
        req.headers.get('X-Dev-Usuario') === 'fantasma'
          ? json({ codigo: 'NAO_AUTENTICADO', error: 'usuário não autenticado', dev: true }, 401)
          : json(base),
      'GET /api/copiloto/estado': () => json({ disponivel: true }),
    });
    const { user } = renderComProvedores(<StudioLayout />);

    expect(await screen.findByRole('alert')).toHaveTextContent('Não reconhecemos seu usuário.');
    expect(screen.queryByText(/usuário não autenticado/)).not.toBeInTheDocument();
    expect(screen.queryByRole('complementary', { name: 'Copiloto' })).not.toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Menu principal' })).not.toBeInTheDocument();

    await user.click(screen.getByRole('combobox', { name: 'Ver como' }));
    await user.click(within(screen.getByRole('listbox')).getByText('Superlógica (admin)'));

    expect(await screen.findByText('Ana')).toBeInTheDocument();
    expect(screen.queryByText('Não reconhecemos seu usuário.')).not.toBeInTheDocument();
  });

  it('hidesSwitchOnUnrecognizedScreenOutsideDev', async () => {
    responderCom({ 'GET /api/eu': () => json({ codigo: 'NAO_AUTENTICADO', error: 'x' }, 401) });

    renderComProvedores(<StudioLayout />);

    expect(await screen.findByText('Não reconhecemos seu usuário.')).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Ver como' })).not.toBeInTheDocument();
  });

  it('hidesSwitchOutsideDev', async () => {
    responderCom({ 'GET /api/eu': () => json({ ...base, dev: false }) });

    renderComProvedores(<StudioLayout />);

    expect(await screen.findByText('Ana')).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Ver como' })).not.toBeInTheDocument();
  });

  it('neverShowsPreviousUserDataAfterSwitchClick', async () => {
    let soltar: () => void = () => {};
    const presa = new Promise<void>((resolve) => {
      soltar = resolve;
    });
    responderCom({
      'GET /api/eu': async (req) => {
        if (req.headers.get('X-Dev-Usuario') !== 'gestor-exemplo') return json(base);
        await presa;
        return json({
          ...base,
          id: 'gestor-exemplo',
          nome: 'Bia',
          papel: 'gestor_administradora',
          permissoes: ['skills.ler'],
          administradoraId: 'adm-1',
        });
      },
    });
    const { user } = renderComProvedores(<StudioLayout />);
    await screen.findByText('Ana');
    await user.click(screen.getByRole('combobox', { name: 'Ver como' }));
    const opcao = within(screen.getByRole('listbox')).getByText('Administradora (gestor)');
    const dadoDoAnteriorNaTela = () =>
      screen.queryByText('Ana') !== null ||
      screen.queryByRole('link', { name: 'Avançado' }) !== null;
    let vazou = false;
    const observador = new MutationObserver(() => {
      vazou ||= dadoDoAnteriorNaTela();
    });

    // fireEvent, e não user.click: síncrono, sem dar tempo a timers antes da checagem.
    fireEvent.click(opcao);
    expect(dadoDoAnteriorNaTela()).toBe(false);
    observador.observe(document.body, { childList: true, subtree: true, characterData: true });
    soltar();

    expect(await screen.findByText('Bia')).toBeInTheDocument();
    observador.disconnect();
    expect(vazou).toBe(false);
  });
});
