import { Button } from '@mui/material';
import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { falharSeHouveRotaAusente, json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CopilotoProvider, useCopiloto } from './CopilotoProvider';
import { EstadoVazio } from './EstadoVazio';
import { PainelDoCopiloto } from './PainelDoCopiloto';

const eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.criar', 'skills.editar_textos'],
  administradoraId: null,
  dev: false,
};

function montar() {
  localStorage.setItem('studio.copilotoAberto', 'true');
  let corpoEnviado: unknown = null;
  const fetchFalso = responderCom({
    'GET /api/eu': () => json(eu),
    'GET /api/copiloto/estado': () => json({ disponivel: true }),
    'POST /api/copiloto': async (req) => {
      corpoEnviado = await req.json();
      return json({ mensagem: 'Entendi.', propostas: [], recusas: [] });
    },
  });
  const resultado = renderComProvedores(
    <CopilotoProvider>
      <PainelDoCopiloto />
    </CopilotoProvider>,
  );
  return { ...resultado, fetchFalso, corpoEnviado: () => corpoEnviado };
}

afterEach(() => {
  localStorage.clear();
  falharSeHouveRotaAusente();
});

/** Troca de tela sem passar por nenhuma outra ação do Copiloto (enviar, abrir/fechar…). */
function BotaoDeNavegacao() {
  const { definirContexto } = useCopiloto();
  return (
    <Button onClick={() => definirContexto({ tela: 'condominio' })}>ir para condomínio</Button>
  );
}

function montarComNavegacao() {
  const fetchFalso = responderCom({ 'GET /api/eu': () => json(eu) });
  const resultado = renderComProvedores(
    <CopilotoProvider>
      <BotaoDeNavegacao />
      <EstadoVazio />
    </CopilotoProvider>,
  );
  return { ...resultado, fetchFalso };
}

describe('EstadoVazio', () => {
  it('rendersTitleAndSuggestionsForScreen', async () => {
    montar();

    expect(await screen.findByText('Como posso ajudar?')).toBeInTheDocument();
    expect(
      await screen.findByRole('button', { name: 'Crie uma skill para reservar a quadra' }),
    ).toBeInTheDocument();
  });

  it('clickingSuggestionSendsIt', async () => {
    const { user, corpoEnviado } = montar();

    const sugestao = await screen.findByRole('button', {
      name: 'Crie uma skill para reservar a quadra',
    });
    await user.click(sugestao);

    expect(await screen.findByText('Entendi.')).toBeInTheDocument();
    expect(corpoEnviado()).toEqual({
      mensagem: 'Crie uma skill para reservar a quadra',
      historico: [],
      contexto: { tela: 'skills' },
    });
  });

  it('updatesSuggestionsAfterNavigatingScreens', async () => {
    const { user } = montarComNavegacao();

    expect(await screen.findByText('Quais skills gravam no sistema?')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'ir para condomínio' }));

    expect(
      await screen.findByText('O que muda deste condomínio para o Padrão?'),
    ).toBeInTheDocument();
    expect(screen.queryByText('Quais skills gravam no sistema?')).not.toBeInTheDocument();
  });
});
