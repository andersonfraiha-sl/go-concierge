import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useSingleFlight } from './useSingleFlight';

describe('useSingleFlight', () => {
  it('startsTheWorkOnTheFirstFlush', async () => {
    const salvar = vi.fn().mockResolvedValue(undefined);
    const { result } = renderHook(() => useSingleFlight(salvar));
    await result.current.flush(() => false);
    expect(salvar).toHaveBeenCalledTimes(1);
  });

  it('waitsForTheRunningWorkInsteadOfStartingASecondOneInParallel', async () => {
    let resolverPrimeira: () => void = () => undefined;
    const primeira = new Promise<void>((resolve) => {
      resolverPrimeira = resolve;
    });
    const salvar = vi.fn().mockReturnValueOnce(primeira).mockResolvedValue(undefined);
    const { result } = renderHook(() => useSingleFlight(salvar));
    void result.current.start();
    const flush = result.current.flush(() => false);
    expect(salvar).toHaveBeenCalledTimes(1);
    resolverPrimeira();
    await flush;
    expect(salvar).toHaveBeenCalledTimes(1);
  });

  it('runsASecondPassOnlyWhenDeNovoSaysSo', async () => {
    const salvar = vi.fn().mockResolvedValue(undefined);
    const { result } = renderHook(() => useSingleFlight(salvar));
    void result.current.start();
    await result.current.flush(() => true);
    expect(salvar).toHaveBeenCalledTimes(2);
  });

  it('doesNotRunASecondPassWhenDeNovoSaysNo', async () => {
    const salvar = vi.fn().mockResolvedValue(undefined);
    const { result } = renderHook(() => useSingleFlight(salvar));
    void result.current.start();
    await result.current.flush(() => false);
    expect(salvar).toHaveBeenCalledTimes(1);
  });
});
