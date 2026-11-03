import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { GuardaAvancado } from './GuardaAvancado';

const eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'gestor',
  permissoes: ['skills.ler'],
  administradoraId: 'adm-1',
  dev: false,
};

describe('GuardaAvancado', () => {
  it('showsNoAccessWithoutPermission', async () => {
    responderCom({ 'GET /api/eu': () => json(eu) });
    renderComProvedores(<GuardaAvancado />);
    expect(await screen.findByText('Você não tem acesso a esta área.')).toBeInTheDocument();
  });

  it('showsPageWithPermission', async () => {
    responderCom({ 'GET /api/eu': () => json({ ...eu, permissoes: ['studio.avancado'] }) });
    renderComProvedores(<GuardaAvancado />);
    expect(await screen.findByRole('heading', { name: 'Avançado' })).toBeInTheDocument();
  });

  it('showsRetryWhenEuFails', async () => {
    responderCom({ 'GET /api/eu': () => json({ codigo: 'X' }, 500) });
    renderComProvedores(<GuardaAvancado />);
    expect(await screen.findByRole('button', { name: 'Tentar de novo' })).toBeInTheDocument();
    expect(screen.queryByText('Você não tem acesso a esta área.')).not.toBeInTheDocument();
  });
});
