import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { responderCom } from '@/test/fetchFalso';
import { construtor, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio } from './paginaDeTeste';

describe('CondominioPage', () => {
  it('showsDefaultWithoutIdentification', async () => {
    responderCom(rotasBase(construtor));
    abrirCondominio('*');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Padrão (todos)' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
  });

  it('gestorCannotEditDefault', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio('*');

    expect(
      await screen.findByText('Vale para todas as administradoras; só a Superlógica muda.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: 'Agendar mudança neste condomínio' })).toBeDisabled();
    expect(screen.getByRole('textbox', { name: 'Para todas as skills' })).toBeDisabled();
  });

  describe('em tela estreita', () => {
    const original = window.matchMedia;
    afterEach(() =>
      Object.defineProperty(window, 'matchMedia', {
        value: original,
        configurable: true,
        writable: true,
      }),
    );

    it('staysReadOnlyBelow900px', async () => {
      Object.defineProperty(window, 'matchMedia', {
        configurable: true,
        writable: true,
        value: (query: string) => ({
          matches: true,
          media: query,
          addEventListener: () => {},
          removeEventListener: () => {},
          addListener: () => {},
          removeListener: () => {},
          dispatchEvent: () => false,
          onchange: null,
        }),
      });
      responderCom(rotasBase(construtor));
      abrirCondominio();

      expect(
        await screen.findByRole('switch', { name: 'Agendar mudança neste condomínio' }),
      ).toBeDisabled();
      expect(screen.getByRole('textbox', { name: 'Para todas as skills' })).toBeDisabled();
      expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
    });
  });
});
