import '@fontsource/inter/400.css';
import '@fontsource/inter/500.css';
import '@fontsource/inter/600.css';
import '@fontsource/plus-jakarta-sans/500.css';
import '@fontsource/plus-jakarta-sans/600.css';
import '@fontsource/plus-jakarta-sans/700.css';
import { CssBaseline, ThemeProvider } from '@mui/material';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { RouterProvider } from 'react-router/dom';
import { criarRotas } from '@/app/router';
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
          <RouterProvider router={criarRotas()} />
        </SnackbarProvider>
      </ThemeProvider>
    </QueryClientProvider>
  </StrictMode>,
);
