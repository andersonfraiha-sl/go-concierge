import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, type RenderResult } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import type { ReactNode } from 'react';
import { createMemoryRouter, RouterProvider } from 'react-router';
import { SnackbarProvider } from '@/shared/snackbar/SnackbarContext';
import { criarTema } from '@/theme/theme';

type Opcoes = {
  /** URL inicial, com query se precisar. */
  rota?: string;
  /** Padrão de rota em que o componente é montado, ex.: "/skills/:nome". */
  caminho?: string;
};

export function renderComProvedores(
  ui: ReactNode,
  { rota = '/', caminho = '*' }: Opcoes = {},
): RenderResult & { user: UserEvent } {
  const cliente = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const roteador = createMemoryRouter([{ path: caminho, element: ui }], { initialEntries: [rota] });
  const resultado = render(
    <QueryClientProvider client={cliente}>
      <ThemeProvider theme={criarTema()}>
        <CssBaseline />
        <SnackbarProvider>
          <RouterProvider router={roteador} />
        </SnackbarProvider>
      </ThemeProvider>
    </QueryClientProvider>,
  );
  return { ...resultado, user: userEvent.setup() };
}
