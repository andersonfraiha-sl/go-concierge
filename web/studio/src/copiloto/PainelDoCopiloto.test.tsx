import { Box } from '@mui/material';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { StudioLayout } from '@/app/StudioLayout';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { tokens } from '@/theme/tokens';
import { CopilotoProvider, useCopiloto, useRealceDoCopiloto } from './CopilotoProvider';

const quebra = vi.hoisted(() => ({ ativa: false }));

vi.mock('./EstadoVazio', async (original) => {
  const modulo = await original<typeof import('./EstadoVazio')>();
  return {
    EstadoVazio: () => {
      if (quebra.ativa) throw new Error('painel quebrou');
      return modulo.EstadoVazio();
    },
  };
});

const eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos', 'skills.excluir', 'condominios.habilitar'],
  administradoraId: null,
  dev: false,
};

const respostaComProposta = {
  mensagem: 'Preparei uma mudança. Confere antes de eu aplicar:',
  propostas: [
    {
      id: '0',
      resumo: 'Ligar Reserva de Área Comum em Vila Verde',
      operacao: {
        tipo: 'habilitar_skill',
        skill: 'reserva',
        condominio: 'vila-verde',
        habilitado: true,
      },
    },
  ],
  recusas: [],
};

function respostaCom(resumo: string, operacao: unknown) {
  return { mensagem: 'Confere:', propostas: [{ id: '0', resumo, operacao }], recusas: [] };
}

const PERSONA = 'Deixar o tom mais gentil';
const respostaDePersona = respostaCom(PERSONA, {
  tipo: 'editar_persona',
  agente: 'agentes/concierge',
  texto: 'Seja gentil.',
});

const EXCLUSAO = 'Excluir Agendar mudança';
const respostaDeExclusao = respostaCom(EXCLUSAO, {
  tipo: 'excluir_skill',
  skill: 'agendar-mudanca',
});

const textosLidos = {
  caminho: 'agentes/concierge/agentes/agendar-mudanca',
  versao: 'v1',
  textos: {
    titulo: 'Agendar mudança',
    quandoUsar: '',
    frasesDeExemplo: [],
    palavrasChave: [],
    temFluxo: true,
  },
};

type Rotas = Parameters<typeof responderCom>[0];

function montar(rotas: Rotas = {}, { disponivel = true, aberto = true } = {}) {
  if (aberto) localStorage.setItem('studio.copilotoAberto', 'true');
  const fetchFalso = responderCom({
    'GET /api/eu': () => json(eu),
    'GET /api/copiloto/estado': () => json({ disponivel }),
    ...rotas,
  });
  return { ...renderComProvedores(<StudioLayout />), fetchFalso };
}

const painel = () => screen.getByRole('complementary', { name: 'Copiloto' });
const caixa = () => screen.getByRole('textbox', { name: 'Mensagem para o copiloto' });

function esperarAborto(req: Request): Promise<Response> {
  return new Promise((_, rejeitar) => {
    req.signal.addEventListener('abort', () => rejeitar(req.signal.reason));
  });
}

afterEach(() => {
  quebra.ativa = false;
});

describe('PainelDoCopiloto', () => {
  it('togglesFromHeaderButton', async () => {
    const { user } = montar({}, { aberto: false });
    const botao = screen.getByRole('button', { name: 'Copiloto' });
    expect(botao).toHaveAttribute('aria-pressed', 'false');
    expect(screen.queryByRole('complementary', { name: 'Copiloto' })).not.toBeInTheDocument();

    await user.click(botao);

    expect(botao).toHaveAttribute('aria-pressed', 'true');
    expect(await within(painel()).findByText('Pronto')).toBeInTheDocument();
    expect(localStorage.getItem('studio.copilotoAberto')).toBe('true');

    await user.click(botao);

    expect(screen.queryByRole('complementary', { name: 'Copiloto' })).not.toBeInTheDocument();
    expect(localStorage.getItem('studio.copilotoAberto')).toBe('false');
  });

  it('sendsOnEnterAndBreaksLineOnShiftEnter', async () => {
    let corpo: unknown = null;
    const { user } = montar({
      'POST /api/copiloto': async (req) => {
        corpo = await req.json();
        return json({ mensagem: 'Entendi.', propostas: [], recusas: [] });
      },
    });

    await user.type(caixa(), 'linha 1{Shift>}{Enter}{/Shift}linha 2');
    expect(corpo).toBeNull();
    await user.type(caixa(), '{Enter}');

    expect(await screen.findByText('Entendi.')).toBeInTheDocument();
    expect(corpo).toEqual({
      mensagem: 'linha 1\nlinha 2',
      historico: [],
      contexto: { tela: 'skills' },
    });
    expect(caixa()).toHaveValue('');
  });

  it('showsThinkingWhileWaiting', async () => {
    let responder: (resposta: Response) => void = () => {};
    const { user } = montar({
      'POST /api/copiloto': () =>
        new Promise<Response>((resolver) => {
          responder = resolver;
        }),
    });

    await user.type(caixa(), 'Oi{Enter}');

    expect(await within(painel()).findByText('Pensando…')).toBeInTheDocument();
    expect(caixa()).toBeDisabled();

    responder(json({ mensagem: 'Olá!', propostas: [], recusas: [] }));

    expect(await screen.findByText('Olá!')).toBeInTheDocument();
    expect(within(painel()).getByText('Pronto')).toBeInTheDocument();
    expect(caixa()).toBeEnabled();
  });

  it('showsUnavailableAndDisablesComposer', async () => {
    montar({}, { disponivel: false });

    expect(await within(painel()).findByText('Indisponível')).toBeInTheDocument();
    expect(
      screen.getByText('O Copiloto está fora do ar. O resto do Studio continua funcionando.'),
    ).toBeInTheDocument();
    expect(caixa()).toBeDisabled();
  });

  it('showsServerErrorAsErrorBubble', async () => {
    const { user } = montar({
      'POST /api/copiloto': () =>
        json({ codigo: 'COPILOTO_DEMOROU', mensagem: 'O Copiloto demorou demais.' }, 504),
    });

    await user.type(caixa(), 'Oi{Enter}');

    expect(await screen.findByText('O Copiloto demorou demais.')).toBeInTheDocument();
    expect(caixa()).toBeEnabled();
  });

  it('showsProposalCards', async () => {
    const { user } = montar({ 'POST /api/copiloto': () => json(respostaComProposta) });

    await user.type(caixa(), 'Liga a reserva no Vila Verde{Enter}');

    const cartao = await screen.findByRole('group', {
      name: 'Ligar Reserva de Área Comum em Vila Verde',
    });
    expect(within(cartao).getByRole('button', { name: 'Aplicar' })).toBeInTheDocument();
    expect(within(cartao).getByRole('button', { name: 'Descartar' })).toBeInTheDocument();
  });

  it('appliesProposalAndMarksCardApplied', async () => {
    const { user } = montar({
      'POST /api/copiloto': () => json(respostaDePersona),
      'PUT /api/arquivo': () => json({ salvo: 'persona.md' }),
    });
    await user.type(caixa(), 'Mais gentil{Enter}');
    const cartao = await screen.findByRole('group', { name: PERSONA });

    await user.click(within(cartao).getByRole('button', { name: 'Aplicar' }));

    expect(await within(cartao).findByText('Aplicada')).toBeInTheDocument();
    expect(within(cartao).queryByRole('button', { name: 'Aplicar' })).not.toBeInTheDocument();
  });

  it('showsApplyErrorOnCardAndKeepsItPending', async () => {
    const { user } = montar({
      'POST /api/copiloto': () => json(respostaDePersona),
      'PUT /api/arquivo': () => json({ codigo: 'X', mensagem: 'Sem permissão na Fundação.' }, 403),
    });
    await user.type(caixa(), 'Mais gentil{Enter}');
    const cartao = await screen.findByRole('group', { name: PERSONA });

    await user.click(within(cartao).getByRole('button', { name: 'Aplicar' }));

    expect(await within(cartao).findByRole('alert')).toHaveTextContent(
      'Sem permissão na Fundação.',
    );
    expect(within(cartao).getByRole('button', { name: 'Aplicar' })).toBeEnabled();
    expect(within(cartao).queryByText('Aplicada')).not.toBeInTheDocument();
  });

  it('asksBeforeDeletingAndKeepsSkillWhenCancelled', async () => {
    const { user, fetchFalso } = montar({
      'POST /api/copiloto': () => json(respostaDeExclusao),
      'GET /api/skills/agendar-mudanca': () => json(textosLidos),
    });
    await user.type(caixa(), 'Apaga a mudança{Enter}');
    const cartao = await screen.findByRole('group', { name: EXCLUSAO });

    await user.click(within(cartao).getByRole('button', { name: 'Aplicar' }));
    const dialogo = await screen.findByRole('dialog');
    expect(dialogo).toHaveTextContent(
      'Excluir "Agendar mudança"? O fluxo e as regras somem junto.',
    );
    await user.click(within(dialogo).getByRole('button', { name: 'Cancelar' }));

    await waitFor(() =>
      expect(within(cartao).getByRole('button', { name: 'Aplicar' })).toBeEnabled(),
    );
    expect(fetchFalso.mock.calls.some(([, init]) => init?.method === 'DELETE')).toBe(false);
    expect(within(cartao).queryByText('Aplicada')).not.toBeInTheDocument();
  });

  it('hidesApplyWhenPersonCannotApply', async () => {
    const { user } = montar({
      'GET /api/eu': () => json({ ...eu, permissoes: ['skills.ler'] }),
      'POST /api/copiloto': () => json(respostaDePersona),
    });
    await user.type(caixa(), 'Mais gentil{Enter}');
    const cartao = await screen.findByRole('group', { name: PERSONA });

    expect(await within(cartao).findByText('Você não pode aplicar esta.')).toBeInTheDocument();
    expect(within(cartao).queryByRole('button', { name: 'Aplicar' })).not.toBeInTheDocument();
  });

  it('discardsOnlyTheChosenProposal', async () => {
    const { user } = montar({ 'POST /api/copiloto': () => json(respostaComProposta) });
    const nome = 'Ligar Reserva de Área Comum em Vila Verde';
    await user.type(caixa(), 'Liga a reserva{Enter}');
    await screen.findByRole('group', { name: nome });
    await user.type(caixa(), 'De novo{Enter}');
    await waitFor(() => expect(screen.getAllByRole('group', { name: nome })).toHaveLength(2));

    const [primeiro] = screen.getAllByRole('group', { name: nome });
    if (!primeiro) throw new Error('sem cartão');
    await user.click(within(primeiro).getByRole('button', { name: 'Descartar' }));

    expect(screen.getAllByRole('group', { name: nome })).toHaveLength(1);
    expect(screen.getAllByText(respostaComProposta.mensagem)).toHaveLength(2);
  });

  it('keepsStudioWorkingWhenCopilotCrashes', async () => {
    quebra.ativa = true;
    vi.spyOn(console, 'error').mockImplementation(() => {});
    const { user } = montar();

    expect(
      await screen.findByText('O Copiloto teve um problema. Feche e abra de novo.'),
    ).toBeInTheDocument();
    expect(await screen.findByRole('link', { name: 'Casos de uso' })).toBeInTheDocument();

    quebra.ativa = false;
    const botao = screen.getByRole('button', { name: 'Copiloto' });
    await user.click(botao);
    await user.click(botao);

    expect(await within(painel()).findByText('Pronto')).toBeInTheDocument();
  });

  it('cancelsRequestOnClose', async () => {
    const { user, fetchFalso } = montar({ 'POST /api/copiloto': esperarAborto });
    await user.type(caixa(), 'Oi{Enter}');
    await within(painel()).findByText('Pensando…');
    const chamada = fetchFalso.mock.calls.find(([, init]) => init?.method === 'POST');
    const sinal = chamada?.[1]?.signal;
    expect(sinal?.aborted).toBe(false);

    await user.click(screen.getByRole('button', { name: 'Copiloto' }));

    await waitFor(() => expect(sinal?.aborted).toBe(true));
  });

  it('abortsRequestOnUnmount', async () => {
    const { user, fetchFalso, unmount } = montar({ 'POST /api/copiloto': esperarAborto });
    await user.type(caixa(), 'Oi{Enter}');
    await within(painel()).findByText('Pensando…');
    const sinal = fetchFalso.mock.calls.find(([, init]) => init?.method === 'POST')?.[1]?.signal;

    unmount();

    expect(sinal?.aborted).toBe(true);
  });

  it('returnsFocusToHeaderButtonWhenClosedFromPanel', async () => {
    const { user } = montar();
    await within(painel()).findByText('Pronto');

    await user.click(screen.getByRole('button', { name: 'Fechar copiloto' }));

    expect(screen.queryByRole('complementary', { name: 'Copiloto' })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Copiloto' })).toHaveFocus();
  });

  it('showsOwnMessageWhenClientDeadlinePasses', async () => {
    const { user } = montar({
      'POST /api/copiloto': () => Promise.reject(new DOMException('prazo', 'TimeoutError')),
    });

    await user.type(caixa(), 'Oi{Enter}');

    expect(await screen.findByText('O Copiloto demorou demais.')).toBeInTheDocument();
    expect(screen.queryByText('Sem conexão com o servidor.')).not.toBeInTheDocument();
  });

  it('highlightsAppliedTargetFor1500ms', () => {
    vi.useFakeTimers();
    try {
      renderComProvedores(
        <CopilotoProvider>
          <CampoRealcavel />
        </CopilotoProvider>,
      );
      const campo = screen.getByRole('button', { name: 'Realçar' });

      fireEvent.click(campo);
      expect(campo).toHaveStyle({ backgroundColor: tokens.color.realceDoCopiloto });

      act(() => vi.advanceTimersByTime(1500));
      expect(campo).not.toHaveStyle({ backgroundColor: tokens.color.realceDoCopiloto });
    } finally {
      vi.useRealTimers();
    }
  });

  it('restartsHighlightWhenTargetIsAppliedAgain', () => {
    vi.useFakeTimers();
    try {
      renderComProvedores(
        <CopilotoProvider>
          <CampoRealcavel />
        </CopilotoProvider>,
      );
      const campo = screen.getByRole('button', { name: 'Realçar' });

      fireEvent.click(campo);
      act(() => vi.advanceTimersByTime(1000));
      fireEvent.click(campo);
      act(() => vi.advanceTimersByTime(1000));

      expect(campo).toHaveStyle({ backgroundColor: tokens.color.realceDoCopiloto });
    } finally {
      vi.useRealTimers();
    }
  });
});

const ALVO = 'skill:mudanca:resumo';

function CampoRealcavel() {
  const { realcar } = useCopiloto();
  return (
    <Box component="button" onClick={() => realcar([ALVO])} sx={useRealceDoCopiloto(ALVO)}>
      Realçar
    </Box>
  );
}
