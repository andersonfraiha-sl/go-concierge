import { screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CabecalhoDeDetalhe } from './CabecalhoDeDetalhe';
import { LayoutDeDetalhe } from './LayoutDeDetalhe';

const eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor_superlogica',
  permissoes: ['skills.ler'],
  administradoraId: null,
  dev: false,
};

function Tela() {
  return (
    <>
      <CabecalhoDeDetalhe
        voltar={{ para: '/skills', rotulo: 'Voltar para Skills' }}
        titulo="Agendar mudança"
        subtitulo="Skills · registra no sistema"
        testarNoConsole
      />
      <p>conteúdo da tela</p>
    </>
  );
}

describe('CabecalhoDeDetalhe no LayoutDeDetalhe', () => {
  it('trocaBarraPeloCabecalhoEMantemOMenu', async () => {
    responderCom({ 'GET /api/eu': () => json(eu) });
    renderComProvedores(<Tela />, { layout: <LayoutDeDetalhe /> });

    const cabecalho = await screen.findByRole('banner');
    expect(within(cabecalho).getByRole('heading', { level: 1 })).toHaveTextContent(
      'Agendar mudança',
    );
    expect(within(cabecalho).getByRole('link', { name: 'Voltar para Skills' })).toHaveAttribute(
      'href',
      '/skills',
    );
    expect(within(cabecalho).getByRole('link', { name: 'Testar no Console' })).toHaveAttribute(
      'href',
      '/console',
    );
    expect(within(cabecalho).getByRole('button', { name: 'Copiloto' })).toBeInTheDocument();
    expect(within(screen.getByRole('main')).getByText('conteúdo da tela')).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Menu principal' })).toBeInTheDocument();
    expect(screen.queryByText(/Salvo às/)).not.toBeInTheDocument();
  });

  it('showsReadOnlyBadgeInsteadOfConsoleForViewers', async () => {
    responderCom({ 'GET /api/eu': () => json(eu) });
    renderComProvedores(
      <CabecalhoDeDetalhe
        voltar={{ para: '/skills', rotulo: 'Voltar para Skills' }}
        titulo="Agendar mudança"
        testarNoConsole
        somenteLeitura
      />,
    );

    const cabecalho = await screen.findByRole('banner');
    expect(within(cabecalho).getByText('Só leitura')).toBeInTheDocument();
    expect(within(cabecalho).queryByRole('link', { name: 'Testar no Console' })).toBeNull();
  });
});
