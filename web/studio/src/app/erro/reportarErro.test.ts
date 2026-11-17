import { afterEach, describe, expect, it, vi } from 'vitest';
import { instalarCapturaGlobal, mascararDocumentos } from './reportarErro';

describe('mascararDocumentos', () => {
  it.each([
    ['cpf 123.456.789-09 inválido', 'cpf ***.***.***-** inválido'],
    ['cpf 12345678909', 'cpf ***.***.***-**'],
    ['tel (11) 98765-4321 ok', 'tel (**) *****-**** ok'],
    ['tel +55 11 98765-4321', 'tel (**) *****-****'],
    ['fixo 11 3456-7890', 'fixo (**) *****-****'],
    ['sem dados, id 42', 'sem dados, id 42'],
  ])('mascara %s', (entrada, saida) => {
    expect(mascararDocumentos(entrada)).toBe(saida);
  });
});

describe('instalarCapturaGlobal', () => {
  afterEach(() => vi.restoreAllMocks());

  it('reportaErrosDeJanelaEPromessasRejeitadas', () => {
    const espiao = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const remover = instalarCapturaGlobal(() => ({ rota: '/x', usuarioId: 'u1', dev: false }));

    window.dispatchEvent(new ErrorEvent('error', { error: new Error('boom 123.456.789-09') }));
    const rejeicao = new Event('unhandledrejection') as PromiseRejectionEvent;
    Object.assign(rejeicao, { reason: new Error('rejeitada') });
    window.dispatchEvent(rejeicao);
    remover();

    expect(espiao).toHaveBeenCalledWith(
      'erro de tela',
      expect.objectContaining({
        origem: 'janela',
        mensagem: 'boom ***.***.***-**',
        usuarioId: 'u1',
      }),
    );
    expect(espiao).toHaveBeenCalledWith(
      'erro de tela',
      expect.objectContaining({ origem: 'promessa', mensagem: 'rejeitada' }),
    );
  });
});
