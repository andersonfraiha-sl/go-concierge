import { cleanup, screen, waitFor, within } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { PADRAO, useEstado } from './contratos';
import {
  camadaDoCondominioFalsa,
  estadoFalso,
  gestor,
  respostaMapeamento,
  rotasBase,
} from './estadoFalso';
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

type Opcoes = { condominio?: string; caso?: string; editavel?: boolean; onFechar?: () => void };

/** Busca o estado ao vivo, como fará a página que abre o drawer, para refletir o que o servidor devolve. */
function Harness({
  condominio = 'vila-verde',
  caso = 'agendar-mudanca',
  editavel = true,
  onFechar = () => {},
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
      onFechar={onFechar}
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

    await user.click(await screen.findByRole('button', { name: 'Síndico' }));

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

    await user.click(await screen.findByRole('button', { name: 'Síndico' }));

    await waitFor(() => expect(corpos[0]).toMatchObject({ papeis: [] }));
    expect(screen.getByRole('button', { name: 'Todos' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('showsDefaultAsPlaceholder', async () => {
    responderCom(rotasBase(gestor));
    abrirDrawer();

    expect(await screen.findByLabelText('Antecedência mínima')).toHaveAttribute('placeholder', '2');
  });

  describe('orientação na camada do condomínio', () => {
    const CAMADA = '/api/skills/agendar-mudanca/condominios/vila-verde';
    const gestorDeSkills: Eu = {
      ...gestor,
      permissoes: [...gestor.permissoes, 'skills.editar_etapas', 'skills.editar_textos'],
    };
    const campo = () => screen.findByRole('textbox', { name: 'Orientação desta skill' });

    /** Camada que muda quando o rascunho é gravado, como o servidor faria. */
    function camadaQueGuarda(orientacao: string, temRascunho: boolean) {
      const corpos: Corpo[] = [];
      const publicacoes: Corpo[] = [];
      let atual = {
        ...camadaDoCondominioFalsa('agendar-mudanca', 'vila-verde', orientacao),
        temRascunho,
      };
      const rotas = {
        [`GET ${CAMADA}`]: () => json(atual),
        [`PUT ${CAMADA}/rascunho`]: async (req: Request) => {
          const corpo = (await req.json()) as Corpo;
          corpos.push(corpo);
          return json({ versao: 'v2', problemas: [] });
        },
        [`POST ${CAMADA}/publicar`]: async (req: Request) => {
          publicacoes.push((await req.json()) as Corpo);
          atual = { ...atual, temRascunho: false };
          return json({ numero: 3 }, 201);
        },
      };
      return { corpos, publicacoes, rotas };
    }

    it('readsOrientationFromCondominioLayer', async () => {
      const { rotas } = camadaQueGuarda('Chamar o zelador.', false);
      responderCom({ ...rotasBase(gestorDeSkills), ...rotas });
      abrirDrawer();

      expect(await campo()).toHaveValue('Chamar o zelador.');
    });

    it('autosavesOrientationToDraft', async () => {
      const { corpos, rotas } = camadaQueGuarda('', false);
      responderCom({ ...rotasBase(gestorDeSkills), ...rotas });
      const { user } = abrirDrawer();

      await user.type(await campo(), 'Chamar o zelador antes.');
      expect(corpos).toHaveLength(0);

      await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
      expect(corpos[0]).toMatchObject({
        versao: 'v1',
        documento: { orientacao: 'Chamar o zelador antes.' },
      });
    });

    it('keepsOtherSectionsWhenSavingOrientation', async () => {
      const { corpos, rotas } = camadaQueGuarda('', false);
      responderCom({ ...rotasBase(gestorDeSkills), ...rotas });
      const { user } = abrirDrawer();

      await user.type(await campo(), 'Avisar a portaria.');
      await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });

      const original = camadaDoCondominioFalsa('agendar-mudanca', 'vila-verde').documento;
      expect(corpos[0]).toMatchObject({
        documento: {
          textos: original.textos,
          ferramentas: original.ferramentas,
          fluxo: original.fluxo,
        },
      });
    });

    it('publishesOrientation', async () => {
      const { publicacoes, rotas } = camadaQueGuarda('Chamar o zelador.', true);
      responderCom({ ...rotasBase(gestorDeSkills), ...rotas });
      const { user } = abrirDrawer();

      await campo();
      await user.click(await screen.findByRole('button', { name: 'Publicar' }));
      const dialogo = await screen.findByRole('dialog', { name: 'Publicar sua versão' });
      await user.type(
        within(dialogo).getByRole('textbox', { name: 'Resumo' }),
        'Zelador primeiro.',
      );
      await user.click(within(dialogo).getByRole('button', { name: 'Publicar' }));

      await waitFor(() => expect(publicacoes).toEqual([{ resumo: 'Zelador primeiro.' }]));
      expect(await screen.findByText('Orientação publicada.')).toBeInTheDocument();
      await waitFor(() =>
        expect(screen.queryByRole('button', { name: 'Publicar' })).not.toBeInTheDocument(),
      );
    });

    it('offersPublishOnlyWithADraft', async () => {
      const { rotas } = camadaQueGuarda('Chamar o zelador.', false);
      responderCom({ ...rotasBase(gestorDeSkills), ...rotas });
      abrirDrawer();

      await campo();
      expect(screen.queryByRole('button', { name: 'Publicar' })).not.toBeInTheDocument();
    });

    it('locksOrientationWithoutPermissionToEditSteps', async () => {
      responderCom(rotasBase(gestor));
      abrirDrawer();

      expect(await campo()).toBeDisabled();
    });

    it('keepsLegacyGuidanceForThePadrao', async () => {
      const { corpos, handlers } = gravador('PUT /api/admin/orientacao');
      responderCom({ ...rotasBase(gestorDeSkills), ...handlers });
      const { user } = abrirDrawer({ condominio: PADRAO });

      await user.type(await screen.findByPlaceholderText(/^Ex\.:/), 'Chamar o zelador antes.');

      await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
      expect(corpos[0]).toEqual({
        condominio: PADRAO,
        geral: '',
        por_caso: { 'agendar-mudanca': 'Chamar o zelador antes.' },
      });
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

    const campo = await screen.findByLabelText('Antecedência mínima');
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

  it('closesOnCloseButtonClick', async () => {
    responderCom(rotasBase(gestor));
    let fechou = false;
    const { user } = renderComProvedores(<Harness onFechar={() => (fechou = true)} />);

    await user.click(await screen.findByRole('button', { name: 'Fechar detalhes' }));

    expect(fechou).toBe(true);
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
    expect(screen.getByRole('button', { name: 'Síndico' })).toBeDisabled();
    expect(await screen.findByPlaceholderText(/^Ex\.:/)).toBeDisabled();
    expect(screen.getByLabelText('Antecedência mínima')).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Voltar ao Padrão' })).not.toBeInTheDocument();
  });
});
