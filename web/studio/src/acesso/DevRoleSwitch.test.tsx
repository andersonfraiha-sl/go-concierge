import { screen, within } from '@testing-library/react';
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

  it('hidesSwitchOutsideDev', async () => {
    responderCom({ 'GET /api/eu': () => json({ ...base, dev: false }) });

    renderComProvedores(<StudioLayout />);

    expect(await screen.findByText('Ana')).toBeInTheDocument();
    expect(screen.queryByRole('combobox', { name: 'Ver como' })).not.toBeInTheDocument();
  });

  it('dropsPreviousUserDataOnSwitch', async () => {
    let chamadas = 0;
    let soltar: () => void = () => {};
    const presa = new Promise<void>((resolve) => {
      soltar = resolve;
    });
    responderCom({
      'GET /api/eu': async () => {
        chamadas += 1;
        if (chamadas === 1) return json(base);
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
    await user.click(within(screen.getByRole('listbox')).getByText('Administradora (gestor)'));

    expect(screen.queryByText('Ana')).not.toBeInTheDocument();
    soltar();
    expect(await screen.findByText('Bia')).toBeInTheDocument();
  });
});
