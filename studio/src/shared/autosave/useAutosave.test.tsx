import { act, renderHook } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { useAutosave } from './useAutosave';

type Props = { valor: string; bloqueio?: string | null };

function montar(salvar: (valor: string) => Promise<void>, inicial: Props = { valor: 'a' }) {
  return renderHook((props: Props) => useAutosave({ salvar, ...props }), { initialProps: inicial });
}

const avancar = (ms: number) => act(() => vi.advanceTimersByTimeAsync(ms));

describe('useAutosave', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('doesNotSaveInitialValue', async () => {
    const salvar = vi.fn().mockResolvedValue(undefined);
    const { result } = montar(salvar);
    await avancar(5000);
    expect(salvar).not.toHaveBeenCalled();
    expect(result.current.estado).toEqual({ tipo: 'em_dia' });
  });

  it('savesOnceOneSecondAfterLastChange', async () => {
    const salvar = vi.fn().mockResolvedValue(undefined);
    const { rerender, result } = montar(salvar);
    rerender({ valor: 'b' });
    await avancar(500);
    rerender({ valor: 'c' });
    await avancar(999);
    expect(salvar).not.toHaveBeenCalled();
    await avancar(1);
    expect(salvar).toHaveBeenCalledTimes(1);
    expect(salvar).toHaveBeenCalledWith('c');
    expect(result.current.estado.tipo).toBe('salvo');
  });

  it('savesEditMadeDuringFlightAfterFirstSave', async () => {
    const resolvers: Array<() => void> = [];
    let simultaneas = 0;
    let maxSimultaneas = 0;
    const salvar = vi.fn((_valor: string) => {
      simultaneas += 1;
      maxSimultaneas = Math.max(maxSimultaneas, simultaneas);
      return new Promise<void>((resolve) => {
        resolvers.push(() => {
          simultaneas -= 1;
          resolve();
        });
      });
    });
    const { rerender } = montar(salvar);
    rerender({ valor: 'b' });
    await avancar(1000);
    expect(salvar).toHaveBeenCalledTimes(1);
    rerender({ valor: 'c' });
    await avancar(1500);
    expect(salvar).toHaveBeenCalledTimes(1);
    resolvers[0]?.();
    await avancar(0);
    expect(salvar).toHaveBeenCalledTimes(2);
    expect(salvar).toHaveBeenLastCalledWith('c');
    resolvers[1]?.();
    await avancar(0);
    expect(maxSimultaneas).toBe(1);
  });

  it('keepsEditAfterFailureAndRetrySaves', async () => {
    const salvar = vi.fn().mockRejectedValueOnce(new Error('caiu')).mockResolvedValue(undefined);
    const { rerender, result } = montar(salvar);
    rerender({ valor: 'b' });
    await avancar(1000);
    expect(result.current.estado).toEqual({ tipo: 'erro', mensagem: 'caiu' });
    rerender({ valor: 'c' });
    await avancar(5000);
    expect(salvar).toHaveBeenCalledTimes(1);
    await act(() => result.current.tentarDeNovo());
    expect(salvar).toHaveBeenCalledTimes(2);
    expect(salvar).toHaveBeenLastCalledWith('c');
    expect(result.current.estado.tipo).toBe('salvo');
  });

  it('doesNotSaveWhileBlocked', async () => {
    const salvar = vi.fn().mockResolvedValue(undefined);
    const { rerender, result } = montar(salvar);
    rerender({ valor: 'b', bloqueio: 'corrija o bloco marcado' });
    await avancar(5000);
    expect(salvar).not.toHaveBeenCalled();
    expect(result.current.estado).toEqual({ tipo: 'bloqueado', motivo: 'corrija o bloco marcado' });
    rerender({ valor: 'b', bloqueio: null });
    await avancar(1000);
    expect(salvar).toHaveBeenCalledWith('b');
  });

  it('flushesPendingEditOnUnmount', async () => {
    const salvar = vi.fn().mockResolvedValue(undefined);
    const { rerender, unmount } = montar(salvar);
    rerender({ valor: 'b' });
    await avancar(200);
    unmount();
    await avancar(0);
    expect(salvar).toHaveBeenCalledTimes(1);
    expect(salvar).toHaveBeenCalledWith('b');
  });

  it('logsFailureOnUnmountFlush', async () => {
    const erro = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const salvar = vi.fn().mockRejectedValue(new Error('caiu'));
    const { rerender, unmount } = montar(salvar);
    rerender({ valor: 'b' });
    unmount();
    await avancar(0);
    expect(erro).toHaveBeenCalled();
    erro.mockRestore();
  });
});
