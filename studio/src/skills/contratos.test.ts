import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { createElement, type ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { paraIdentificador, SkillsSchema, TextosLidosSchema, useFluxo } from './contratos';

describe('paraIdentificador', () => {
  it.each([
    { titulo: 'Reserva do Salão', esperado: 'reserva-do-salao' },
    { titulo: '  Código  de Retirada!', esperado: 'codigo-de-retirada' },
    { titulo: 'Ação 2', esperado: 'acao-2' },
    { titulo: '---', esperado: '' },
    { titulo: '', esperado: '' },
  ])('converte "$titulo" em "$esperado"', ({ titulo, esperado }) => {
    expect(paraIdentificador(titulo)).toBe(esperado);
  });
});

describe('contratos de skills', () => {
  it('leOsTextosDaSkill', () => {
    const lido = TextosLidosSchema.parse({
      caminho: 'agentes/concierge/agentes/agendar-mudanca',
      textos: {
        titulo: 'Agendamento de Mudança',
        quandoUsar: 'Usado para o morador agendar a mudança.',
        frasesDeExemplo: ['quero agendar minha mudança'],
        palavrasChave: ['mudança'],
        temFluxo: true,
      },
      versao: '3f9c1a7e5b2d4086',
    });
    expect(lido.textos.frasesDeExemplo).toEqual(['quero agendar minha mudança']);
  });

  it('leASkillSemListasQuandoGoMandaNull', () => {
    const lido = SkillsSchema.parse({
      assistente: 'agentes/concierge',
      skills: [
        {
          nome: 'faq',
          id: '',
          titulo: 'faq',
          descricao: 'Dúvidas',
          escrita: false,
          tipo: 'conhecimento',
          recursos: null,
          caminho: 'agentes/concierge/agentes/faq',
        },
      ],
    });
    expect(lido.skills[0]?.recursos).toEqual([]);
  });

  it('recusaSkillSemNome', () => {
    expect(SkillsSchema.safeParse({ assistente: 'x', skills: [{}] }).success).toBe(false);
  });
});

describe('useFluxo', () => {
  /** Cliente com as opções padrão do TanStack Query: é o retry de fábrica que está em teste. */
  const comClientePadrao = () => {
    const cliente = new QueryClient();
    return ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client: cliente }, children);
  };

  it('doesNotRetryWhenSkillHasNoFlow', async () => {
    const falso = responderCom({
      'GET /api/skills/faq/fluxo': () => json({ codigo: 'SEM_FLUXO', mensagem: 'Skill sem fluxo.' }, 404),
    });
    const { result } = renderHook(() => useFluxo('faq'), { wrapper: comClientePadrao() });
    await waitFor(() => expect(result.current.isError).toBe(true), { timeout: 500 });
    expect(falso).toHaveBeenCalledTimes(1);
  });

  it('retriesOtherFailures', async () => {
    responderCom({ 'GET /api/skills/faq/fluxo': () => json({ codigo: 'ERRO', mensagem: 'Caiu.' }, 500) });
    const { result } = renderHook(() => useFluxo('faq'), { wrapper: comClientePadrao() });
    await waitFor(() => expect(result.current.failureCount).toBe(1));
    expect(result.current.isPending).toBe(true);
  });
});
