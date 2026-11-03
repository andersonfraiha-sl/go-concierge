import { Alert, Snackbar, type SnackbarCloseReason } from '@mui/material';
import { createContext, useContext, useState, type ReactNode } from 'react';

type Tipo = 'success' | 'error' | 'info';
type Aviso = { mensagem: string; tipo: Tipo };
type SnackbarContextValue = { mostrar: (mensagem: string, tipo?: Tipo) => void };

const DURACAO_MS = 4000;

const SnackbarContext = createContext<SnackbarContextValue | null>(null);

export function useSnackbar(): SnackbarContextValue {
  const contexto = useContext(SnackbarContext);
  if (!contexto) throw new Error('useSnackbar fora do SnackbarProvider');
  return contexto;
}

export function SnackbarProvider({ children }: { children: ReactNode }) {
  const [aviso, setAviso] = useState<Aviso | null>(null);
  const [aberto, setAberto] = useState(false);

  const mostrar = (mensagem: string, tipo: Tipo = 'success') => {
    setAviso({ mensagem, tipo });
    setAberto(true);
  };
  const fechar = () => setAberto(false);
  // Clique fora não fecha: o aviso dura os 4000 ms inteiros.
  const aoFecharSnackbar = (_: unknown, motivo: SnackbarCloseReason) => {
    if (motivo !== 'clickaway') fechar();
  };

  return (
    <SnackbarContext.Provider value={{ mostrar }}>
      {children}
      <Snackbar
        open={aberto}
        autoHideDuration={DURACAO_MS}
        onClose={aoFecharSnackbar}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert onClose={fechar} severity={aviso?.tipo ?? 'success'} variant="filled">
          {aviso?.mensagem}
        </Alert>
      </Snackbar>
    </SnackbarContext.Provider>
  );
}
