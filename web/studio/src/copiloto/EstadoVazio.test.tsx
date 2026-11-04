import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';
import { falharSeHouveRotaAusente, json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CopilotoProvider } from './CopilotoProvider';
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
});
