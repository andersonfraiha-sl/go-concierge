import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderHook, waitFor } from '@testing-library/react';
import { createElement, type ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { camadaDaRota } from './camadaDaRota';
import {
  AtualizacoesSchema,
  CamadaLidaSchema,
  caminhoDaCamada,
  DiferencaSchema,
  decidir,
  marcarCiente,
  publicarBase,
  useCamada,
} from './contratos';

const textos = {
  titulo: 'Reserva do salão',
  quandoUsar: 'Quando o morador quer reservar.',
  frasesDeExemplo: ['quero reservar o salão'],
  palavrasChave: ['salão'],
  temFluxo: true,
};

const camadaLida = {
  camada: { skill: 'reserva', tipo: 'administradora', id: 'adm-1' },
  documento: {
    textos,
    fluxo: {
      blocos: [{ id: 'e-1a2b3c4d', tipo: 'confirmar', texto: 'Confirma?' }],
      informacoes: null,
    },
    ferramentas: ['reservas'],
    orientacao: '',
  },
  versao: 'abc123',
  temRascunho: false,
  publicada: { numero: 3, autor: 'Ana', criadoEm: '2026-11-04T10:00:00Z' },
  ferramentasDaBase: ['reservas'],
  problemas: null,
};

const diferenca = {
  trecho: 'etapa:e-1a2b3c4d',
  tipo: 'etapa',
  rotulo: 'Confirmar reserva',
  situacao: 'conflito',
  obrigatorio: false,
  antes: null,
  depois: '{"id":"e-1a2b3c4d"}',
  minha: null,
};

describe('contratos das camadas', () => {
  it('parsesLayerExample', () => {
    const lida = CamadaLidaSchema.parse(camadaLida);
    expect(lida.documento.fluxo.blocos).toHaveLength(1);
    expect(lida.documento.fluxo.informacoes).toEqual([]);
    expect(lida.problemas).toEqual([]);
    expect(lida.publicada?.numero).toBe(3);
  });

  it('parsesLayerWithoutPublishedVersion', () => {
    expect(CamadaLidaSchema.parse({ ...camadaLida, publicada: null }).publicada).toBeNull();
  });

  it('parsesDifferenceWithNullSides', () => {
    const lida = DiferencaSchema.parse(diferenca);
    expect([lida.antes, lida.minha]).toEqual([null, null]);
  });

  it('rejectsUnknownSituation', () => {
    expect(DiferencaSchema.safeParse({ ...diferenca, situacao: 'talvez' }).success).toBe(false);
  });

  it('parsesUpdatesWithNotices', () => {
    const lidas = AtualizacoesSchema.parse({
      atualizacoes: [
        {
          id: 'u1',
          camada: { skill: 'reserva', tipo: 'condominio', id: 'c-1' },
          estado: 'pendente',
          para: { numero: 4, resumo: 'Novo texto' },
          prazoEm: null,
          trechos: 3,
          conflitos: 1,
        },
      ],
      avisos: null,
    });
    expect(lidas.atualizacoes[0]?.conflitos).toBe(1);
    expect(lidas.avisos).toEqual([]);
  });

  it('buildsLayerPath', () => {
    expect(caminhoDaCamada('reserva', 'administradora', 'adm-1')).toBe(
      '/api/skills/reserva/administradoras/adm-1',
    );
    expect(caminhoDaCamada('reserva', 'condominio', 'c 1')).toBe(
      '/api/skills/reserva/condominios/c%201',
    );
  });

  it('readsLayerFromRoute', () => {
    expect(camadaDaRota({ nome: 'reserva', tipo: 'condominio', id: 'c-1' })).toEqual({
      skill: 'reserva',
      tipo: 'condominio',
      id: 'c-1',
    });
  });

  it.each([
    { nome: 'reserva', tipo: 'bloco', id: 'x' },
    { nome: 'reserva', tipo: undefined, id: 'x' },
    { nome: undefined, tipo: 'condominio', id: 'x' },
    { nome: 'reserva', tipo: 'condominio', id: '' },
  ])('rejectsUnknownLayerType: $tipo / $nome / $id', (parametros) => {
    expect(camadaDaRota(parametros)).toBeNull();
  });
});

describe('chamadas das camadas', () => {
  const comCliente = ({ children }: { children: ReactNode }) =>
    createElement(
      QueryClientProvider,
      { client: new QueryClient({ defaultOptions: { queries: { retry: false } } }) },
      children,
    );

  it('readsLayerThroughTheApi', async () => {
    responderCom({ 'GET /api/skills/reserva/administradoras/adm-1': () => json(camadaLida) });
    const { result } = renderHook(() => useCamada('reserva', 'administradora', 'adm-1'), {
      wrapper: comCliente,
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.versao).toBe('abc123');
  });

  it('sendsDecisionsWithTheSeenHead', async () => {
    let corpo: unknown;
    responderCom({
      'POST /api/atualizacoes/u1/decisao': async (req) => {
        corpo = await req.json();
        return json({ numero: 5, estado: 'parcial' }, 201);
      },
    });
    const decisoes = [{ trecho: 'texto:titulo', escolha: 'aceitar' } as const];
    const decidida = await decidir('u1', { cabeca: 4, decisoes });
    expect(decidida.estado).toBe('parcial');
    expect(corpo).toEqual({ cabeca: 4, decisoes });
  });

  it('publishesBaseAndAcknowledgesNotice', async () => {
    responderCom({
      'POST /api/skills/reserva/publicar': () => json({ numero: 2, camadas: 6 }, 201),
      'POST /api/avisos/a1/ciente': () => new Response(null, { status: 204 }),
    });
    const publicada = await publicarBase('reserva', {
      resumo: 'r',
      obrigatorios: [],
      motivo: '',
      encerraEm: null,
    });
    expect(publicada).toEqual({ numero: 2, camadas: 6 });
    expect(await marcarCiente('a1')).toBeNull();
  });
});
