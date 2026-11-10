/** O React Flow só mede os nós no callback do ResizeObserver; sem medida, nem nós visíveis nem arestas. */
export function medirNoJsdom(): void {
  class ResizeObserverQueMede {
    constructor(private readonly callback: ResizeObserverCallback) {}
    observe(target: Element): void {
      const contentRect = {
        x: 0,
        y: 0,
        top: 0,
        left: 0,
        width: 800,
        height: 600,
        bottom: 600,
        right: 800,
      };
      // Depois do commit: o React Flow só guarda o DOM do canvas no efeito do pai, que roda após o dos nós.
      setTimeout(() => {
        if (target.isConnected)
          this.callback(
            [{ target, contentRect } as ResizeObserverEntry],
            this as unknown as ResizeObserver,
          );
      }, 0);
    }
    unobserve(): void {}
    disconnect(): void {}
  }
  for (const alvo of [globalThis, window]) {
    Object.defineProperty(alvo, 'ResizeObserver', {
      value: ResizeObserverQueMede,
      configurable: true,
    });
  }
  Object.defineProperties(HTMLElement.prototype, {
    offsetHeight: { configurable: true, get: () => 600 },
    offsetWidth: { configurable: true, get: () => 800 },
  });
}
