import { screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { StudioLayout } from './StudioLayout';

const eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'condominios.editar', 'studio.avancado'],
  administradoraId: null,
  dev: false,
};

describe('StudioLayout', () => {
  beforeEach(() => {
    responderCom({ 'GET /api/eu': () => json(eu) });
  });

  it('mostraStudioNoTopoESeisItensDeNavegacao', async () => {
    renderComProvedores(<StudioLayout />);

    expect(screen.getByText('Studio')).toBeInTheDocument();
    await screen.findByRole('link', { name: 'Skills' });
    const rotulos = ['Concierge', 'Skills', 'Ferramentas', 'Condomínios', 'Avançado', 'Atendente'];
    for (const rotulo of rotulos) {
      expect(screen.getByRole('link', { name: rotulo })).toBeInTheDocument();
    }
  });

  it('apontaAtendenteParaOConsoleForaDoSpa', () => {
    renderComProvedores(<StudioLayout />);
    expect(screen.getByRole('link', { name: 'Atendente' })).toHaveAttribute('href', '/console');
  });

  it('iniciaRecolhidoQuandoHaPreferenciaGravada', () => {
    localStorage.setItem('studio.menuRecolhido', 'true');

    renderComProvedores(<StudioLayout />);

    expect(screen.getByRole('button', { name: 'Expandir menu' })).toBeInTheDocument();
  });

  it('recolheOMenuELembraAPreferencia', async () => {
    const { user } = renderComProvedores(<StudioLayout />);

    await user.click(screen.getByRole('button', { name: 'Recolher menu' }));

    expect(screen.getByRole('button', { name: 'Expandir menu' })).toBeInTheDocument();
    expect(localStorage.getItem('studio.menuRecolhido')).toBe('true');
  });
});
