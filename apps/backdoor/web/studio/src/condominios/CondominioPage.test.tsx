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
      await screen.findByRole('heading', { level: 1, name: 'Conteúdo global' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
  });

  it('gestorEditaConteudoDaEmpresaSemHabilitacaoLocal', async () => {
    responderCom(rotasBase(gestor));
    abrirCondominio('*');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Conteúdo da empresa' }),
    ).toBeInTheDocument();
    expect(
      await screen.findByRole('textbox', { name: 'Orientações para este condomínio' }),
    ).toHaveAttribute('aria-readonly', 'false');
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
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
        await screen.findByRole('textbox', { name: 'Orientações para este condomínio' }),
      ).toHaveAttribute('aria-readonly', 'true');
      expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
    });
  });
});
