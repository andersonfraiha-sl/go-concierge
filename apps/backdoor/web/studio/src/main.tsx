import '@fontsource/inter/400.css';
import '@fontsource/inter/500.css';
import '@fontsource/inter/600.css';
import '@fontsource/plus-jakarta-sans/500.css';
import '@fontsource/plus-jakarta-sans/600.css';
import '@fontsource/plus-jakarta-sans/700.css';
import '@fontsource/jetbrains-mono/400.css';
import '@fontsource/jetbrains-mono/500.css';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router/dom';
import type { Eu } from '@/acesso/eu';
import { instalarCapturaGlobal } from '@/app/erro/reportarErro';
import { criarRotas } from '@/app/router';
import { deveTentarDeNovo } from '@/shared/api/http';
import { SnackbarProvider } from '@/shared/snackbar/SnackbarContext';
import { criarTema } from '@/theme/theme';

const queryClient = new QueryClient({ defaultOptions: { queries: { retry: deveTentarDeNovo } } });

instalarCapturaGlobal(() => {
  const eu = queryClient.getQueriesData<Eu>({ queryKey: ['eu'] })[0]?.[1];
  return { rota: window.location.pathname, usuarioId: eu?.id ?? null, dev: eu?.dev ?? false };
});

const raiz = document.getElementById('root');
if (!raiz) throw new Error('elemento #root ausente no index.html');

createRoot(raiz).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <ThemeProvider theme={criarTema()}>
        <CssBaseline />
        <SnackbarProvider>
          <RouterProvider router={criarRotas()} />
        </SnackbarProvider>
      </ThemeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
