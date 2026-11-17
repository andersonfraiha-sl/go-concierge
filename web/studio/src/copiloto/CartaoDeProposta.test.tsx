import { QueryClient } from '@tanstack/react-query';
import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { z } from 'zod';
import { StudioLayout } from '@/app/StudioLayout';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';

const eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.criar', 'skills.editar_textos', 'condominios.habilitar'],
  administradoraId: null,
  dev: false,
};

type Rotas = Parameters<typeof responderCom>[0];

function montar(propostas: unknown[], rotas: Rotas) {
  localStorage.setItem('studio.copilotoAberto', 'true');
  const fetchFalso = responderCom({
    'GET /api/eu': () => json(eu),
    'GET /api/copiloto/estado': () => json({ disponivel: true }),
    'POST /api/copiloto': () => json({ mensagem: 'Confere:', propostas, recusas: [] }),
    ...rotas,
  });
  return { ...renderComProvedores(<StudioLayout />), fetchFalso };
}

async function pedir(user: ReturnType<typeof montar>['user']) {
  const caixa = screen.getByRole('textbox', { name: 'Mensagem para o copiloto' });
  await user.type(caixa, 'Faz{Enter}');
}

const cartao = (resumo: string) => screen.findByRole('group', { name: resumo });
const aplicarEm = (grupo: HTMLElement) =>
  within(grupo).getByRole('button', { name: /Aplicar|Aguarde a anterior|Aplicando/ });

const AjustesSchema = z.object({
  ajustes: z.record(z.string(), z.record(z.string(), z.json())),
});

/** Servidor de ajustes com estado: o GET devolve o que o último PUT gravou. */
function servidorDeAjustes() {
  let payloads: Record<string, Record<string, unknown>> = { 'reserva.criar': { antecedencia: 2 } };
  let soltarPrimeiro = () => {};
  const primeiroPreso = new Promise<void>((soltar) => {
    soltarPrimeiro = soltar;
  });
  let gravacoes = 0;
  const mapeamento = () => ({
    administradoras: [],
    condominios: [],
    habilitacoes: {},
    payloads: { 'vila-verde': payloads },
    orientacoes: {},
  });
  const rotas: Rotas = {
    'GET /api/admin/estado': () =>
      json({ mapeamento: mapeamento(), administradoras: [], casos: [], recursos: [], papeis: [] }),
    'PUT /api/admin/ajustes': async (req) => {
      const { ajustes } = AjustesSchema.parse(await req.json());
      gravacoes += 1;
      if (gravacoes === 1) await primeiroPreso;
      payloads = ajustes;
      return json({ mapeamento: mapeamento(), administradoras: [] });
    },
  };
  return { rotas, soltarPrimeiro, payloads: () => payloads };
}

const ajuste = (id: string, resumo: string, campos: Record<string, number>) => ({
  id,
  resumo,
  operacao: { tipo: 'ajustar', condominio: 'vila-verde', recurso: 'reserva.criar', campos },
});

describe('CartaoDeProposta', () => {
  it('appliesOneProposalAtATimeKeepingBothChanges', async () => {
    const servidor = servidorDeAjustes();
    const { user } = montar(
      [
        ajuste('0', 'Limite de 3 reservas', { limite: 3 }),
        ajuste('1', 'Antecedência de 5 dias', { antecedencia: 5 }),
      ],
      servidor.rotas,
    );
    await pedir(user);
    const primeiro = await cartao('Limite de 3 reservas');
    const segundo = await cartao('Antecedência de 5 dias');

    await user.click(aplicarEm(primeiro));
    expect(
      await within(segundo).findByRole('button', { name: 'Aguarde a anterior' }),
    ).toBeDisabled();
    servidor.soltarPrimeiro();
    expect(await within(primeiro).findByText('Aplicada')).toBeInTheDocument();
    await user.click(await within(segundo).findByRole('button', { name: 'Aplicar' }));

    expect(await within(segundo).findByText('Aplicada')).toBeInTheDocument();
    expect(servidor.payloads()).toEqual({ 'reserva.criar': { antecedencia: 5, limite: 3 } });
  });

  it('retriesOnlyTheTextsWhenCreatedSkillLostThem', async () => {
    const invalidar = vi.spyOn(QueryClient.prototype, 'invalidateQueries');
    let criacoes = 0;
    let gravacoesDeTextos = 0;
    const { user } = montar(
      [
        {
          id: '0',
          resumo: 'Criar Portaria',
          operacao: {
            tipo: 'criar_skill',
            titulo: 'Portaria',
            textos: { resumo: 'Abre o portão.' },
          },
        },
      ],
      {
        'POST /api/skills': () => {
          criacoes += 1;
          return json({ nome: 'portaria' });
        },
        'GET /api/skills/portaria': () =>
          json({
            caminho: 'agentes/concierge/agentes/portaria',
            versao: 'v1',
            textos: { titulo: 'Portaria', quandoUsar: '', temFluxo: false },
          }),
        'PUT /api/skills/portaria/textos': () => {
          gravacoesDeTextos += 1;
          return gravacoesDeTextos === 1
            ? json({ mensagem: 'Disco cheio.' }, 500)
            : json({ versao: 'v2' });
        },
      },
    );
    await pedir(user);
    const grupo = await cartao('Criar Portaria');

    await user.click(aplicarEm(grupo));
    expect(await within(grupo).findByRole('alert')).toHaveTextContent(
      'Skill criada, mas os textos não salvaram',
    );
    expect(invalidar).toHaveBeenCalledWith({ queryKey: ['skills'] });
    await user.click(aplicarEm(grupo));

    expect(await within(grupo).findByText('Aplicada')).toBeInTheDocument();
    expect(criacoes).toBe(1);
    expect(gravacoesDeTextos).toBe(2);
  });
});
