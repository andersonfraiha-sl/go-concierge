import { Box } from '@mui/material';
import { screen, waitFor, within } from '@testing-library/react';
import type { UserEvent } from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { ArquivosAba } from '@/avancado/ArquivosAba';
import { ConciergePage } from '@/concierge/ConciergePage';
import { CondominioPage } from '@/condominios/CondominioPage';
import { CondominiosPage } from '@/condominios/CondominiosPage';
import { estadoFalso, respostaMapeamento, skillsFalsas } from '@/condominios/estadoFalso';
import { FerramentasPage } from '@/ferramentas/FerramentasPage';
import fixtureDoFluxo from '@/fluxo/modelo/agendar-mudanca.fixture.json';
import { SkillEditorPage } from '@/skills/SkillEditorPage';
import { SkillsPage } from '@/skills/SkillsPage';
import { digitarNoFim } from '@/test/editorRico';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { tokens } from '@/theme/tokens';
import { PainelDoCopiloto } from './PainelDoCopiloto';

type Rota = (req: Request) => Response | Promise<Response>;
type Rotas = Record<string, Rota>;

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: [
    'skills.ler',
    'skills.editar_textos',
    'condominios.habilitar',
    'condominios.editar',
    'studio.avancado',
  ],
  administradoraId: null,
  dev: false,
};
const gestor: Eu = {
  ...construtor,
  papel: 'gestor',
  permissoes: ['skills.ler', 'condominios.habilitar'],
  administradoraId: 'adm-exemplo',
};

const textosLidos = (resumo: string, versao = 'v1') => ({
  caminho: 'agentes/concierge/agentes/agendar-mudanca',
  versao,
  textos: {
    titulo: 'Agendar mudança',
    resumo,
    quandoUsar: '',
    frasesDeExemplo: [],
    palavrasChave: [],
    temFluxo: true,
  },
});

const agente = {
  agente: {
    caminho: 'concierge',
    nome: 'concierge',
    descricao: '',
    persona: 'Voz.',
    tipo: 'orquestrador',
    tipo_declarado: true,
    arquivos: { rules: [{ caminho: 'rules/politicas.md', bytes: 12 }] },
    herdados: {},
    problemas: [],
  },
  recursos: [],
};

const noConcierge = {
  caminho: 'concierge',
  nome: 'concierge',
  tipo: 'agente',
  descricao: '',
  caso: false,
  erros: 0,
  avisos: 0,
  subagentes: [],
};

const RESPOSTA_SEM_PROPOSTA = { mensagem: 'Entendi.', propostas: [], recusas: [] };

const respostaCom = (resumo: string, operacao: unknown) => ({
  mensagem: 'Confere:',
  propostas: [{ id: '0', resumo, operacao }],
  recusas: [],
});

type Montagem = { tela: ReactNode; rota?: string; caminho?: string; eu?: Eu; rotas?: Rotas };

/** Tela com o painel aberto; devolve cada corpo enviado a POST /api/copiloto. */
function montar({ tela, rota = '/', caminho = '*', eu = construtor, rotas = {} }: Montagem) {
  localStorage.setItem('studio.copilotoAberto', 'true');
  const pedidos: Array<{ contexto: unknown }> = [];
  responderCom({
    'GET /api/eu': () => json(eu),
    'GET /api/copiloto/estado': () => json({ disponivel: true }),
    'POST /api/copiloto': async (req) => {
      pedidos.push(await req.json());
      return json(RESPOSTA_SEM_PROPOSTA);
    },
    ...rotas,
  });
  const { user } = renderComProvedores(
    <Box sx={{ display: 'flex' }}>
      <PainelDoCopiloto />
      <Box>{tela}</Box>
    </Box>,
    { rota, caminho },
  );
  return { user, pedidos };
}

const caixa = () => screen.getByRole('textbox', { name: 'Mensagem para o copiloto' });

async function contextoEnviado(user: UserEvent, pedidos: Array<{ contexto: unknown }>) {
  await waitFor(() => expect(caixa()).toBeEnabled());
  await user.type(caixa(), 'Oi{Enter}');
  await waitFor(() => expect(pedidos).toHaveLength(1));
  return pedidos[0]?.contexto;
}

async function aplicarProposta(user: UserEvent, resumo: string) {
  await waitFor(() => expect(caixa()).toBeEnabled());
  await user.type(caixa(), 'Faça{Enter}');
  const cartao = await screen.findByRole('group', { name: resumo });
  await user.click(within(cartao).getByRole('button', { name: 'Aplicar' }));
  await within(cartao).findByText('Aplicada');
}

const rotasDoCondominio = (estado: unknown = estadoFalso): Rotas => ({
  'GET /api/admin/estado': () => json(estado),
  'GET /api/skills': () => json(skillsFalsas),
});

describe('contexto do Copiloto por tela', () => {
  it('sendsSkillsHubContext', async () => {
    const { user, pedidos } = montar({
      tela: <SkillsPage />,
      rotas: { 'GET /api/skills': () => json(skillsFalsas) },
    });
    await screen.findByRole('article', { name: 'Agendar mudança' });

    expect(await contextoEnviado(user, pedidos)).toEqual({ tela: 'skills' });
  });

  it('sendsSkillEditorContext', async () => {
    const { user, pedidos } = montar({
      tela: <SkillEditorPage />,
      rota: '/skills/agendar-mudanca',
      caminho: '/skills/:nome',
      rotas: {
        'GET /api/skills': () => json(skillsFalsas),
        'GET /api/skills/agendar-mudanca': () => json(textosLidos('Agenda.')),
        'GET /api/skills/agendar-mudanca/fluxo': () => json(fixtureDoFluxo),
        'GET /api/admin/estado': () => json(estadoFalso),
      },
    });
    await screen.findByRole('textbox', { name: 'Resumo' });

    expect(await contextoEnviado(user, pedidos)).toEqual({
      tela: 'skill',
      skill: 'agendar-mudanca',
    });
  });

  it('sendsFlowContext', async () => {
    const { user, pedidos } = montar({
      tela: <SkillEditorPage />,
      rota: '/skills/agendar-mudanca?aba=fluxo',
      caminho: '/skills/:nome',
      rotas: {
        'GET /api/skills': () => json(skillsFalsas),
        'GET /api/skills/agendar-mudanca': () => json(textosLidos('Agenda.')),
        'GET /api/skills/agendar-mudanca/fluxo': () => json(fixtureDoFluxo),
        'GET /api/admin/estado': () => json(estadoFalso),
        'GET /api/nexus/recursos': () =>
          json({ account: 'a', origem: 'instantaneo', conferido: true, recursos: [] }),
      },
    });
    await screen.findByRole('region', { name: 'Fluxo' });

    expect(await contextoEnviado(user, pedidos)).toEqual({
      tela: 'fluxo',
      skill: 'agendar-mudanca',
    });
  });

  it('sendsCondominiosContext', async () => {
    const { user, pedidos } = montar({ tela: <CondominiosPage />, rotas: rotasDoCondominio() });
    await screen.findByRole('link', { name: 'Vila Verde' });

    expect(await contextoEnviado(user, pedidos)).toEqual({ tela: 'condominios' });
  });

  it('sendsCondominioContext', async () => {
    const { user, pedidos } = montar({
      tela: <CondominioPage />,
      rota: '/condominios/vila-verde',
      caminho: '/condominios/:id',
      rotas: rotasDoCondominio(),
    });
    await screen.findByRole('heading', { name: 'Vila Verde' });

    expect(await contextoEnviado(user, pedidos)).toEqual({
      tela: 'condominio',
      condominio: 'vila-verde',
    });
  });

  it('sendsConciergeContext', async () => {
    const { user, pedidos } = montar({
      tela: <ConciergePage />,
      rotas: {
        'GET /api/skills': () => json({ assistente: 'concierge', skills: [] }),
        'GET /api/agente': () => json(agente),
        'GET /api/arquivo': () => json({ codigo: 'NAO_ENCONTRADO', mensagem: 'sem arquivo' }, 404),
      },
    });
    await screen.findByRole('textbox', { name: 'Texto da persona' });

    expect(await contextoEnviado(user, pedidos)).toEqual({ tela: 'concierge' });
  });

  it('sendsToolsContext', async () => {
    const { user, pedidos } = montar({
      tela: <FerramentasPage />,
      rotas: {
        'GET /api/nexus/recursos': () =>
          json({ account: 'a', origem: 'instantaneo', conferido: true, recursos: [] }),
        'GET /api/skills': () => json({ assistente: 'concierge', skills: [] }),
      },
    });
    await screen.findByRole('heading', { name: 'Recursos' });

    expect(await contextoEnviado(user, pedidos)).toEqual({ tela: 'ferramentas' });
  });

  it('sendsAdvancedFileContext', async () => {
    const { user, pedidos } = montar({
      tela: <ArquivosAba no={noConcierge} editar />,
      rotas: {
        'GET /api/agente': () => json(agente),
        'GET /api/arquivo': () => json({ caminho: 'rules/politicas.md', conteudo: 'Regra' }),
      },
    });
    await user.click(await screen.findByRole('button', { name: 'politicas.md' }));
    await screen.findByDisplayValue('Regra');

    expect(await contextoEnviado(user, pedidos)).toEqual({
      tela: 'avancado',
      agente: 'concierge',
      caminho: 'rules/politicas.md',
    });
  });

  it('omitsAdvancedContextWithoutPermission', async () => {
    const { user, pedidos } = montar({
      tela: <ArquivosAba no={noConcierge} editar={false} />,
      eu: gestor,
      rotas: {
        'GET /api/agente': () => json(agente),
        'GET /api/arquivo': () => json({ caminho: 'rules/politicas.md', conteudo: 'Regra' }),
      },
    });
    await user.click(await screen.findByRole('button', { name: 'politicas.md' }));
    await screen.findByDisplayValue('Regra');

    expect(await contextoEnviado(user, pedidos)).toEqual({ tela: 'skills' });
  });
});

describe('realce e formulário aberto quando o Copiloto aplica', () => {
  it('highlightsAppliedSkillRow', async () => {
    const ligada = structuredClone(estadoFalso);
    ligada.mapeamento.habilitacoes['vila-verde'] = {
      'reserva-do-salao': { habilitado: true, papeis: null },
    };
    let estado: unknown = estadoFalso;
    const resumo = 'Ligar Reserva do salão em Vila Verde';
    const operacao = {
      tipo: 'habilitar_skill',
      condominio: 'vila-verde',
      skill: 'reserva-do-salao',
      habilitado: true,
    };
    const { user } = montar({
      tela: <CondominioPage />,
      rota: '/condominios/vila-verde',
      caminho: '/condominios/:id',
      rotas: {
        ...rotasDoCondominio(),
        'GET /api/admin/estado': () => json(estado),
        'POST /api/copiloto': () => json(respostaCom(resumo, operacao)),
        'PUT /api/admin/habilitacao': () => {
          estado = ligada;
          return json({ mapeamento: ligada.mapeamento, administradoras: ligada.administradoras });
        },
      },
    });
    await screen.findByRole('heading', { name: 'Vila Verde' });

    await aplicarProposta(user, resumo);

    const linha = screen.getByRole('switch', { name: 'Reserva do salão neste condomínio' });
    await waitFor(() => expect(linha).toBeChecked());
    expect(linha.closest('[role="button"]')).toHaveStyle({
      backgroundColor: tokens.color.realceDoCopiloto,
    });
  });

  it('highlightsAppliedSummaryField', async () => {
    let lido = textosLidos('Agenda.');
    const resumo = 'Melhorar o resumo';
    const operacao = {
      tipo: 'preencher_textos',
      skill: 'agendar-mudanca',
      textos: { resumo: 'Marca o dia da mudança.' },
    };
    const { user } = montar({
      tela: <SkillEditorPage />,
      rota: '/skills/agendar-mudanca',
      caminho: '/skills/:nome',
      rotas: {
        'GET /api/skills/agendar-mudanca': () => json(lido),
        'GET /api/skills': () => json(skillsFalsas),
        'GET /api/skills/agendar-mudanca/fluxo': () => json(fixtureDoFluxo),
        'GET /api/admin/estado': () => json(estadoFalso),
        'POST /api/copiloto': () => json(respostaCom(resumo, operacao)),
        'PUT /api/skills/agendar-mudanca/textos': () => {
          lido = textosLidos('Marca o dia da mudança.', 'v2');
          return json({ versao: 'v2' });
        },
      },
    });
    await screen.findByDisplayValue('Agenda.');

    await aplicarProposta(user, resumo);

    // Campo intocado: volta ao que o servidor gravou, sem aviso.
    const campo = await screen.findByDisplayValue('Marca o dia da mudança.');
    expect(campo.closest('.MuiInputBase-root')).toHaveStyle({
      backgroundColor: tokens.color.realceDoCopiloto,
    });
    expect(screen.queryByText(/O Copiloto mudou/)).not.toBeInTheDocument();
  });

  /** Edição da pessoa pendente (o salvamento automático falhou) quando o Copiloto aplica a orientação. */
  async function conflitoNaOrientacao() {
    const comOrientacao = structuredClone(estadoFalso);
    comOrientacao.mapeamento.orientacoes = {
      'vila-verde': { geral: 'Piscina até 23h.', por_caso: {} },
    };
    comOrientacao.mapeamento.atualizado_em = '2026-11-04T10:00:00Z';
    let estado: unknown = estadoFalso;
    const corpos: Array<{ geral: string }> = [];
    const resumo = 'Escrever a orientação da piscina';
    const operacao = { tipo: 'orientar', condominio: 'vila-verde', texto: 'Piscina até 23h.' };
    const { user } = montar({
      tela: <CondominioPage />,
      rota: '/condominios/vila-verde',
      caminho: '/condominios/:id',
      rotas: {
        ...rotasDoCondominio(),
        'GET /api/admin/estado': () => json(estado),
        'POST /api/copiloto': () => json(respostaCom(resumo, operacao)),
        // A primeira é o salvamento automático da pessoa, que falha e deixa a edição pendente.
        'PUT /api/admin/orientacao': async (req) => {
          corpos.push(await req.json());
          if (corpos.length === 1) return json({ codigo: 'ERRO', mensagem: 'Caiu.' }, 500);
          estado = comOrientacao;
          return json({ ...respostaMapeamento, mapeamento: comOrientacao.mapeamento });
        },
      },
    });
    const campo = await screen.findByRole('textbox', { name: 'Orientações para este condomínio' });
    await digitarNoFim(user, campo, 'Sem mudança aos domingos.');
    await screen.findByText('Falha ao salvar', {}, { timeout: 3000 });

    await aplicarProposta(user, resumo);

    expect(await screen.findByText(/O Copiloto mudou este campo/)).toBeInTheDocument();
    expect(screen.getByText('Não salvo: escolha qual versão fica')).toBeInTheDocument();
    expect(campo).toHaveTextContent(/^Sem mudança aos domingos\.$/);
    expect(corpos).toHaveLength(2);
    return { user, campo, corpos };
  }

  it('holdsSaveAndUsesCopilotVersionWhenChosen', async () => {
    const { user, campo, corpos } = await conflitoNaOrientacao();

    await user.click(screen.getByRole('button', { name: 'Usar o do Copiloto' }));

    await waitFor(() => expect(campo).toHaveTextContent(/^Piscina até 23h\.$/));
    expect(screen.queryByText(/O Copiloto mudou este campo/)).not.toBeInTheDocument();
    expect(corpos).toHaveLength(2);
  });

  it('holdsSaveAndKeepsUserTextWhenChosen', async () => {
    const { user, campo, corpos } = await conflitoNaOrientacao();

    await user.click(screen.getByRole('button', { name: 'Manter o meu' }));
    await user.click(screen.getByRole('button', { name: 'Tentar de novo' }));

    await waitFor(() => expect(corpos).toHaveLength(3));
    expect(corpos[2]).toMatchObject({ geral: 'Sem mudança aos domingos.' });
    expect(campo).toHaveTextContent(/^Sem mudança aos domingos\.$/);
  });
});
