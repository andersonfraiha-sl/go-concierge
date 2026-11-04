import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { OrquestracaoAba } from './OrquestracaoAba';

const orquestracao = {
  agente: 'concierge',
  arquivo: { modo: 'router', bind_confidence: 0.8, max_itens: 5, max_iteracoes: 8 },
  efetivo: { modo: 'router', bind_confidence: 0.8, max_itens: 5, max_iteracoes: 8 },
  ambiente: '',
};

describe('OrquestracaoAba', () => {
  it('showsHelpForEachLimit', async () => {
    responderCom({ 'GET /api/admin/orquestracao': () => json(orquestracao) });
    renderComProvedores(<OrquestracaoAba editar={true} />);
    expect(
      await screen.findByText(
        'Abaixo disso, o item vira pergunta de confirmação ao morador antes de executar (0,5 a 1).',
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Quantos pedidos o plano aceita de uma vez (1 a 10).'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Passos de pensar e agir por mensagem; no limite, junta o que já tem (1 a 15).',
      ),
    ).toBeInTheDocument();
  });

  it('showsWhenToUseEachMode', async () => {
    responderCom({ 'GET /api/admin/orquestracao': () => json(orquestracao) });
    renderComProvedores(<OrquestracaoAba editar={true} />);
    expect(await screen.findByText('Quando: Pedidos de um item só.')).toBeInTheDocument();
    expect(
      screen.getByText('Quando: Mensagens com vários pedidos; menos idas e vindas.'),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Quando: Pedidos abertos, em que o próximo passo depende do resultado do anterior.',
      ),
    ).toBeInTheDocument();
  });
});
