import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderComProvedores } from '@/test/render';
import { StudioLayout } from './StudioLayout';

describe('StudioLayout', () => {
  it('mostraStudioNoTopoESeisItensDeNavegacao', () => {
    renderComProvedores(<StudioLayout />);

    expect(screen.getByText('Studio')).toBeInTheDocument();
    const rotulos = ['Skills', 'Assistente', 'Ferramentas', 'Condomínios', 'Avançado', 'Atendente'];
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
