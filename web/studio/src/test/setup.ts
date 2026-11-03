import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import { falharSeHouveRotaAusente } from './fetchFalso';

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
Object.defineProperty(globalThis, 'ResizeObserver', { value: ResizeObserverFalso, configurable: true });
Object.defineProperty(globalThis, 'DOMMatrixReadOnly', { value: DOMMatrixReadOnlyFalso, configurable: true });
Object.defineProperty(window, 'matchMedia', { value: nadaCasa, configurable: true, writable: true });

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
  falharSeHouveRotaAusente();
});
