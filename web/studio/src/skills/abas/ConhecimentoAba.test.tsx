import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { caminhoDeDocumento } from '@/catalogo/contratos';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { ConhecimentoAba } from './ConhecimentoAba';
import { AGENTE, agenteLido, arquivo, construtor, gestor } from './fixturesDeAbas';

describe('ConhecimentoAba', () => {
  it('showsInheritedDocsReadOnly', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/agente': () =>
        json(
          agenteLido({
            documentos: [
              arquivo(caminhoDeDocumento('faq.md')),
              arquivo(caminhoDeDocumento('tabela.pdf'), 2048),
            ],
            herdados: [arquivo(caminhoDeDocumento('regras-gerais.md'), 512)],
          }),
        ),
      'GET /api/arquivo': () =>
        json({ caminho: caminhoDeDocumento('faq.md'), conteudo: 'Perguntas.' }),
    });
    renderComProvedores(<ConhecimentoAba nome="agendar-mudanca" caminho={AGENTE} />);

    expect(await screen.findByRole('textbox', { name: 'Faq' })).toHaveValue('Perguntas.');
    expect(screen.getByText('Regras gerais')).toBeInTheDocument();
    expect(screen.getByText('Vem do Concierge')).toBeInTheDocument();
    expect(screen.getByText('tabela.pdf')).toBeInTheDocument();
    expect(screen.getByText('2 KB')).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Regras gerais' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Excluir Regras gerais' })).not.toBeInTheDocument();
  });

  const faq = caminhoDeDocumento('faq.md');
  const montar = () =>
    renderComProvedores(<ConhecimentoAba nome="agendar-mudanca" caminho={AGENTE} />);
  const rotasBase = () => ({
    'GET /api/eu': () => json(construtor),
    'GET /api/agente': () => json(agenteLido({ documentos: [arquivo(faq)] })),
    'GET /api/arquivo': () => json({ caminho: faq, conteudo: 'Perguntas.' }),
  });

  it('createsDocument', async () => {
    const corpos: unknown[] = [];
    responderCom({
      ...rotasBase(),
      'PUT /api/arquivo': async (req: Request) => {
        corpos.push(await req.json());
        return json({ salvo: 'ok' });
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Novo documento' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Guia da mudança');
    await user.click(screen.getByRole('button', { name: 'Criar' }));
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos).toEqual([
      { agente: AGENTE, caminho: caminhoDeDocumento('guia-da-mudanca.md'), conteudo: '' },
    ]);
  });

  it('deletesDocumentAfterConfirmation', async () => {
    const apagados: string[] = [];
    responderCom({
      ...rotasBase(),
      'DELETE /api/arquivo': (req: Request) => {
        apagados.push(new URL(req.url).searchParams.get('caminho') ?? '');
        return json({ excluido: faq });
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Excluir Faq' }));
    expect(screen.getByText('Excluir "Faq"? A skill deixa de consultá-lo.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Excluir' }));
    await waitFor(() => expect(apagados).toEqual([faq]));
  });

  it('retriesAfterLoadError', async () => {
    let falhar = true;
    responderCom({
      ...rotasBase(),
      'GET /api/agente': () =>
        falhar
          ? json({ codigo: 'ERRO', mensagem: 'boom' }, 500)
          : json(agenteLido({ documentos: [arquivo(faq)] })),
    });
    const { user } = montar();
    const tentar = await screen.findByRole('button', { name: 'Tentar de novo' });
    falhar = false;
    await user.click(tentar);
    expect(await screen.findByRole('textbox', { name: 'Faq' })).toBeInTheDocument();
  });

  it('gestorReadsOnly', async () => {
    responderCom({ ...rotasBase(), 'GET /api/eu': () => json(gestor) });
    montar();
    expect(await screen.findByRole('textbox', { name: 'Faq' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Novo documento' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Excluir Faq' })).not.toBeInTheDocument();
  });

  it('titledBaseComumForConcierge', async () => {
    responderCom(rotasBase());
    renderComProvedores(<ConhecimentoAba caminho={AGENTE} titulo="Base de conhecimento comum" />);
    expect(
      await screen.findByRole('heading', { name: 'Base de conhecimento comum' }),
    ).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Base de conhecimento' })).not.toBeInTheDocument();
    expect(await screen.findByRole('textbox', { name: 'Faq' })).toHaveValue('Perguntas.');
  });
});
