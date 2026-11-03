import { cleanup, screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { PADRAO, useEstado } from './contratos';
import { estadoFalso, gestor, respostaMapeamento, rotasBase } from './estadoFalso';
import { SkillDoCondominioDrawer } from './SkillDoCondominioDrawer';

type Corpo = Record<string, unknown>;

const donoDoCatalogo: Eu = {
  id: 'u3',
  nome: 'Caio',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
  administradoraId: null,
  dev: true,
};

type Opcoes = { condominio?: string; caso?: string; editavel?: boolean };

/** Busca o estado ao vivo, como fará a página que abre o drawer, para refletir o que o servidor devolve. */
function Harness({
  condominio = 'vila-verde',
  caso = 'agendar-mudanca',
  editavel = true,
}: Opcoes): ReactNode {
  const consulta = useEstado();
  if (!consulta.data) return null;
  const encontrado = consulta.data.casos.find((c) => c.nome === caso);
  if (!encontrado) throw new Error(`caso não encontrado: ${caso}`);
  return (
    <SkillDoCondominioDrawer
      estado={consulta.data}
      condominio={condominio}
      caso={encontrado}
      titulo={encontrado.titulo}
      editavel={editavel}
      onFechar={() => {}}
    />
  );
}

const abrirDrawer = (opcoes: Opcoes = {}) => renderComProvedores(<Harness {...opcoes} />);

/** Captura o corpo de toda gravação na rota dada e devolve a resposta padrão de mapeamento. */
function gravador(rota: string): {
  corpos: Corpo[];
  handlers: Record<string, (req: Request) => Promise<Response>>;
} {
  const corpos: Corpo[] = [];
  const handlers = {
    [rota]: async (req: Request) => {
      corpos.push((await req.json()) as Corpo);
      return json(respostaMapeamento);
    },
  };
  return { corpos, handlers };
}

describe('SkillDoCondominioDrawer', () => {
  it.each([
    {
      nome: 'decisão própria',
      condominio: PADRAO,
      caso: 'reserva-do-salao',
      frase: 'Este condomínio tem decisão própria.',
    },
    {
      nome: 'herdada do Padrão',
      condominio: 'vila-verde',
      caso: 'reserva-do-salao',
      frase: 'Sem decisão própria: vale o Padrão. Mudar aqui cria a decisão deste condomínio.',
    },
    {
      nome: 'sem decisão nenhuma',
      condominio: 'vila-verde',
      caso: 'agendar-mudanca',
      frase: 'Sem decisão: ligada para todos.',
    },
  ])('showsOriginOfDecision: $nome', async ({ condominio, caso, frase }) => {
    responderCom(rotasBase(gestor));
    abrirDrawer({ condominio, caso });

    expect(await screen.findByText(frase)).toBeInTheDocument();
  });

  it('restrictsToChosenRoles', async () => {
    const corpos: Corpo[] = [];
    responderCom({
      ...rotasBase(gestor),
      'PUT /api/admin/habilitacao': async (req) => {
        corpos.push((await req.json()) as Corpo);
        const atualizado = structuredClone(estadoFalso);
        atualizado.mapeamento.habilitacoes['vila-verde'] = {
          'agendar-mudanca': { habilitado: true, papeis: ['sindico'] },
        } as never;
        return json({
          mapeamento: atualizado.mapeamento,
          administradoras: atualizado.administradoras,
        });
      },
    });
    const { user } = abrirDrawer();

    await user.click(await screen.findByRole('button', { name: 'síndico' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({
        condominio: 'vila-verde',
        caso: 'agendar-mudanca',
        papeis: ['sindico'],
      }),
    );
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Todos' })).toHaveAttribute(
        'aria-pressed',
        'false',
      ),
    );
  });

  it('deselectingLastRoleFallsBackToTodos', async () => {
    const comSindico = structuredClone(estadoFalso);
    comSindico.mapeamento.habilitacoes['vila-verde'] = {
      'agendar-mudanca': { habilitado: true, papeis: ['sindico'] },
    } as never;
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comSindico),
      ...handlers,
    });
    const { user } = abrirDrawer();

    await user.click(await screen.findByRole('button', { name: 'síndico' }));

    await waitFor(() => expect(corpos[0]).toMatchObject({ papeis: [] }));
    expect(screen.getByRole('button', { name: 'Todos' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('showsDefaultAsPlaceholder', async () => {
    responderCom(rotasBase(gestor));
    abrirDrawer();

    expect(await screen.findByText('Padrão: 2')).toBeInTheDocument();
  });

  it('savesSkillGuidanceKeepingGeneral', async () => {
    const comOrientacao = structuredClone(estadoFalso);
    comOrientacao.mapeamento.orientacoes = {
      'vila-verde': { geral: 'Piscina fecha às 22h.', por_caso: {} },
    };
    const { corpos, handlers } = gravador('PUT /api/admin/orientacao');
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comOrientacao),
      ...handlers,
    });
    const { user } = abrirDrawer();

    await user.type(await screen.findByPlaceholderText(/^Ex\.:/), 'Chamar o zelador antes.');
    expect(corpos).toHaveLength(0);

    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos[0]).toEqual({
      condominio: 'vila-verde',
      geral: 'Piscina fecha às 22h.',
      por_caso: { 'agendar-mudanca': 'Chamar o zelador antes.' },
    });
  });

  it('savesSkillAdjustmentsKeepingOthers', async () => {
    const comAjustes = structuredClone(estadoFalso);
    comAjustes.mapeamento.payloads = {
      'vila-verde': { 'horario-portaria': { horario: '8h às 20h' } },
    };
    const { corpos, handlers } = gravador('PUT /api/admin/ajustes');
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comAjustes),
      ...handlers,
    });
    const { user } = abrirDrawer();

    const campo = await screen.findByLabelText('Antecedência mínima (dias)');
    await user.clear(campo);
    await user.type(campo, '5');

    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos[0]).toMatchObject({
      condominio: 'vila-verde',
      ajustes: {
        'horario-portaria': { horario: '8h às 20h' },
        'agenda-mudanca': { antecedencia_dias: 5 },
      },
    });
  });

  it('resetsToDefault', async () => {
    const comDecisao = structuredClone(estadoFalso);
    comDecisao.mapeamento.habilitacoes['vila-verde'] = {
      'agendar-mudanca': { habilitado: false, papeis: [] },
    } as never;
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    responderCom({
      ...rotasBase(gestor),
      'GET /api/admin/estado': () => json(comDecisao),
      ...handlers,
    });
    const { user } = abrirDrawer();

    await user.click(await screen.findByRole('button', { name: 'Voltar ao Padrão' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({
        condominio: 'vila-verde',
        caso: 'agendar-mudanca',
        limpar: true,
      }),
    );
  });

  it('hidesWhenToUseFromGestor', async () => {
    responderCom(rotasBase(donoDoCatalogo));
    abrirDrawer();
    expect(await screen.findByText('Como o concierge reconhece o pedido')).toBeInTheDocument();
    cleanup();

    responderCom(rotasBase(gestor));
    abrirDrawer();
    await screen.findByText('Agendar mudança');
    expect(screen.queryByText('Como o concierge reconhece o pedido')).not.toBeInTheDocument();
  });

  it('linksToFlow', async () => {
    responderCom(rotasBase(donoDoCatalogo));
    abrirDrawer();

    const link = await screen.findByRole('link', { name: 'Ver o fluxo' });
    expect(link).toHaveAttribute('href', '/skills/agendar-mudanca?aba=fluxo');
  });

  it('disablesEverythingWhenReadOnly', async () => {
    const comDecisao = structuredClone(estadoFalso);
    comDecisao.mapeamento.habilitacoes['vila-verde'] = {
      'agendar-mudanca': { habilitado: true, papeis: [] },
    } as never;
    responderCom({ ...rotasBase(gestor), 'GET /api/admin/estado': () => json(comDecisao) });
    abrirDrawer({ editavel: false });

    expect(await screen.findByRole('switch', { name: 'Ligada neste condomínio' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Todos' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'síndico' })).toBeDisabled();
    expect(screen.getByPlaceholderText(/^Ex\.:/)).toBeDisabled();
    expect(screen.getByLabelText('Antecedência mínima (dias)')).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Voltar ao Padrão' })).not.toBeInTheDocument();
  });
});
