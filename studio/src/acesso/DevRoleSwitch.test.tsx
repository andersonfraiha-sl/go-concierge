import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { StudioLayout } from '@/app/StudioLayout';
import { CHAVE_PAPEL_DEV } from '@/shared/api/http';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import type { Eu } from './eu';

const base: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'condominios.editar', 'studio.avancado'],
  administradoraId: null,
  dev: true,
};

describe('acesso na barra lateral e no topo', () => {
  it('hidesAdvancedForGestor', async () => {
    const gestor: Eu = { ...base, papel: 'gestor', permissoes: ['skills.ler'], administradoraId: 'adm-1' };
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

  it('switchesRoleAndRefetches', async () => {
    const falso = responderCom({ 'GET /api/eu': () => json(base) });
    const { user } = renderComProvedores(<StudioLayout />);
    await screen.findByRole('combobox', { name: 'Ver como' });
    const antes = falso.mock.calls.length;

    await user.click(screen.getByRole('combobox', { name: 'Ver como' }));
    await user.click(within(screen.getByRole('listbox')).getByText('Administradora (gestor)'));

    expect(localStorage.getItem(CHAVE_PAPEL_DEV)).toBe('gestor');
    await screen.findByRole('combobox', { name: 'Ver como' });
    await vi.waitFor(() => expect(falso.mock.calls.length).toBeGreaterThan(antes));
    const ultima = falso.mock.calls.at(-1);
    expect(new Headers(ultima?.[1]?.headers).get('X-Studio-Papel')).toBe('gestor');
  });

  it('showsRoleFromServerNotFromStorage', async () => {
    localStorage.setItem(CHAVE_PAPEL_DEV, 'construtor');
    responderCom({ 'GET /api/eu': () => json({ ...base, papel: 'gestor_avancado' }) });

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
});
