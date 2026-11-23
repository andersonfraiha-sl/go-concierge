import { ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { lazy, Suspense } from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { CopilotoProvider } from '@/copiloto/CopilotoProvider';
import { json, responderCom } from '@/test/fetchFalso';
import { criarTema } from '@/theme/theme';
import { StudioLayout } from '../StudioLayout';
import { LimiteDeRota } from './LimiteDeErro';

const eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: [],
  administradoraId: null,
  dev: false,
};

let quebrar = true;
function Quebradica() {
  if (quebrar) throw new Error('falha de render');
  return <p>tela ok</p>;
}
const ChunkQuebrado = lazy(() =>
  Promise.reject(new Error('Failed to fetch dynamically imported module: /assets/x.js')),
);

function renderizar(elemento: React.ReactNode) {
  const rotas = createMemoryRouter(
    [
      {
        path: '/',
        element: (
          <CopilotoProvider>
            <StudioLayout />
          </CopilotoProvider>
        ),
        children: [{ element: <LimiteDeRota />, children: [{ path: 'x', element: elemento }] }],
      },
    ],
    { initialEntries: ['/x'] },
  );
  render(
    <QueryClientProvider
      client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}
    >
      <ThemeProvider theme={criarTema()}>
        <RouterProvider router={rotas} />
      </ThemeProvider>
    </QueryClientProvider>,
  );
}

describe('LimiteDeRota', () => {
  beforeEach(() => {
    quebrar = true;
    responderCom({ 'GET /api/eu': () => json(eu) });
    vi.spyOn(console, 'error').mockImplementation(() => undefined);
  });
  afterEach(() => vi.restoreAllMocks());

  it('mostraTelaDeErroMantendoMenuETopbar', async () => {
    renderizar(<Quebradica />);

    expect(await screen.findByText('Algo deu errado nesta tela')).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: 'Menu principal' })).toBeInTheDocument();
    expect(screen.getByText('Studio')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Voltar ao início' })).toBeInTheDocument();
  });

  it('registraErroEstruturadoComStackEComponentStack', async () => {
    renderizar(<Quebradica />);
    await screen.findByText('Algo deu errado nesta tela');

    expect(console.error).toHaveBeenCalledWith(
      'erro de tela',
      expect.objectContaining({
        origem: 'react',
        rota: '/x',
        mensagem: 'falha de render',
        stack: expect.stringContaining('falha de render'),
        componentStack: expect.stringContaining('Quebradica'),
      }),
    );
  });

  it('recuperaAoTentarDeNovo', async () => {
    renderizar(<Quebradica />);
    await screen.findByText('Algo deu errado nesta tela');
    quebrar = false;

    await userEvent.click(screen.getByRole('button', { name: 'Tentar de novo' }));

    expect(await screen.findByText('tela ok')).toBeInTheDocument();
  });

  it('ofereceRecarregarQuandoChunkFalha', async () => {
    renderizar(
      <Suspense>
        <ChunkQuebrado />
      </Suspense>,
    );

    expect(await screen.findByRole('button', { name: 'Recarregar a página' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Tentar de novo' })).not.toBeInTheDocument();
  });

  it('mostraDetalhesSoEmDev', async () => {
    renderizar(<Quebradica />);
    expect(await screen.findByText('Detalhes do erro')).toBeInTheDocument();
  });

  it('escondeDetalhesEmProducao', async () => {
    vi.stubEnv('DEV', false);
    renderizar(<Quebradica />);
    await screen.findByText('Algo deu errado nesta tela');
    expect(screen.queryByText('Detalhes do erro')).not.toBeInTheDocument();
    vi.unstubAllEnvs();
  });
});
