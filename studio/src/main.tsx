import '@fontsource/lato/400.css';
import '@fontsource/lato/700.css';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router/dom';
import { criarRoteador } from '@/app/router';
import { SnackbarProvider } from '@/shared/snackbar/SnackbarContext';
import { criarTema } from '@/theme/theme';

const raiz = document.getElementById('root');
if (!raiz) throw new Error('elemento #root ausente no index.html');

createRoot(raiz).render(
  <StrictMode>
    <QueryClientProvider client={new QueryClient()}>
      <ThemeProvider theme={criarTema()}>
        <CssBaseline />
        <SnackbarProvider>
          <RouterProvider router={criarRoteador()} />
        </SnackbarProvider>
      </ThemeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
