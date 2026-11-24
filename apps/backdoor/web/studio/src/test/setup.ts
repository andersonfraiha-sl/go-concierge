import '@testing-library/jest-dom/vitest';
import { cleanup, configure } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import { falharSeHouveRotaAusente } from './fetchFalso';

// O primeiro render do React Flow passa de 1 s com a suíte inteira em paralelo; em máquinas
// mais carregadas (68 workers concorrentes) até 3 s estourava, gerando falhas instáveis.
configure({ asyncUtilTimeout: 6000 });

class ResizeObserverFalso {
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
}

class DOMMatrixReadOnlyFalso {
  m22 = 1;
}

const nadaCasa = (query: string): MediaQueryList => ({
  matches: false,
  media: query,
  onchange: null,
  addEventListener: () => {},
  removeEventListener: () => {},
  addListener: () => {},
  removeListener: () => {},
  dispatchEvent: () => false,
});

// defineProperty, e não vi.stubGlobal: unstubAllGlobals não pode desfazer estes mocks.
Object.defineProperty(globalThis, 'ResizeObserver', {
  value: ResizeObserverFalso,
  configurable: true,
});
Object.defineProperty(globalThis, 'DOMMatrixReadOnly', {
  value: DOMMatrixReadOnlyFalso,
  configurable: true,
});
Object.defineProperty(window, 'matchMedia', {
  value: nadaCasa,
  configurable: true,
  writable: true,
});
// O editor rico (ProseMirror) mede a seleção ao digitar; o jsdom não tem layout para medir.
Object.defineProperty(document, 'elementFromPoint', { value: () => null, configurable: true });
Object.defineProperty(Range.prototype, 'getClientRects', {
  value: () => [],
  configurable: true,
});
Object.defineProperty(Range.prototype, 'getBoundingClientRect', {
  value: () => new DOMRect(),
  configurable: true,
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
  falharSeHouveRotaAusente();
});
