import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { caminhoDeDocumento } from '@/catalogo/contratos';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { ConhecimentoAba } from './ConhecimentoAba';
import { AGENTE, agenteLido, arquivo, construtor } from './fixturesDeAbas';

describe('ConhecimentoAba', () => {
  it('showsInheritedDocsReadOnly', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/agente': () =>
        json(
          agenteLido({
            documentos: [arquivo(caminhoDeDocumento('faq.md')), arquivo(caminhoDeDocumento('tabela.pdf'), 2048)],
            herdados: [arquivo(caminhoDeDocumento('regras-gerais.md'), 512)],
          }),
        ),
      'GET /api/arquivo': () => json({ caminho: caminhoDeDocumento('faq.md'), conteudo: 'Perguntas.' }),
    });
    renderComProvedores(<ConhecimentoAba nome="agendar-mudanca" caminho={AGENTE} />);

    expect(await screen.findByRole('textbox', { name: 'Faq' })).toHaveValue('Perguntas.');
    expect(screen.getByText('Regras gerais')).toBeInTheDocument();
    expect(screen.getByText('Vem do Assistente')).toBeInTheDocument();
    expect(screen.getByText('tabela.pdf')).toBeInTheDocument();
    expect(screen.getByText('2 KB')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Regras gerais' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Excluir Regras gerais' })).not.toBeInTheDocument();
  });
});
