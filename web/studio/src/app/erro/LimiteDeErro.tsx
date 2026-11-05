import { Alert, Box, Button, Link, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { Component, type ErrorInfo, type ReactNode } from 'react';
import { Outlet, Link as RouterLink, useLocation, useRouteError } from 'react-router';
import { useEu } from '@/acesso/eu';
import { FONTE_MONO } from '@/theme/tokens';
import { falhaDeChunk, reportarErro } from './reportarErro';

type Props = { children: ReactNode; aoCapturar: (erro: unknown, info: ErrorInfo) => void };
type Estado = { erro: unknown; houveErro: boolean };

class Limite extends Component<Props, Estado> {
  state: Estado = { erro: null, houveErro: false };

  static getDerivedStateFromError(erro: unknown): Estado {
    return { erro, houveErro: true };
  }

  componentDidCatch(erro: unknown, info: ErrorInfo) {
    this.props.aoCapturar(erro, info);
  }

  render() {
    if (!this.state.houveErro) return this.props.children;
    return (
      <TelaDeErro
        erro={this.state.erro}
        aoTentarDeNovo={() => this.setState({ erro: null, houveErro: false })}
      />
    );
  }
}

/** Envolve o conteúdo das rotas: menu e topbar ficam, só a área principal mostra o erro. */
export function LimiteDeRota(): ReactNode {
  const { pathname } = useLocation();
  const { data: eu } = useEu();
  const aoCapturar = (erro: unknown, info: ErrorInfo) =>
    reportarErro(erro, {
      origem: 'react',
      rota: pathname,
      usuarioId: eu?.id ?? null,
      dev: eu?.dev ?? false,
      componentStack: info.componentStack ?? undefined,
    });
  return (
    <Limite key={pathname} aoCapturar={aoCapturar}>
      <Outlet />
    </Limite>
  );
}

/** errorElement da raiz: pega o que escapa do layout ou do próprio router. */
export function ErroDaRaiz(): ReactNode {
  const erro = useRouteError();
  const { pathname } = useLocation();
  reportarUmaVez(erro, pathname);
  return (
    <Box sx={{ p: 3 }}>
      <TelaDeErro erro={erro} aoTentarDeNovo={() => window.location.reload()} />
    </Box>
  );
}

// O render pode repetir; o objeto do erro é o mesmo, então reporta só na primeira.
const jaReportados = new WeakSet<object>();
function reportarUmaVez(erro: unknown, rota: string): void {
  if (typeof erro === 'object' && erro !== null) {
    if (jaReportados.has(erro)) return;
    jaReportados.add(erro);
  }
  reportarErro(erro, { origem: 'react', rota, usuarioId: null, dev: false });
}

type TelaProps = { erro: unknown; aoTentarDeNovo: () => void };

export function TelaDeErro({ erro, aoTentarDeNovo }: TelaProps): ReactNode {
  const queryClient = useQueryClient();
  const chunk = falhaDeChunk(erro);
  const tentar = () => {
    void queryClient.refetchQueries({ type: 'active' });
    aoTentarDeNovo();
  };
  return (
    <Box role="alert" sx={{ display: 'flex', flexDirection: 'column', gap: 2, maxWidth: 640 }}>
      <Typography variant="h5" component="h1">
        Algo deu errado nesta tela
      </Typography>
      <Typography color="textSecondary">
        {chunk
          ? 'O Studio foi atualizado e esta tela precisa ser recarregada.'
          : 'Não conseguimos mostrar esta tela agora. Você pode tentar de novo ou voltar ao início.'}
      </Typography>
      <Box sx={{ display: 'flex', gap: 2, alignItems: 'center' }}>
        {chunk ? (
          <Button variant="contained" onClick={() => window.location.reload()}>
            Recarregar a página
          </Button>
        ) : (
          <Button variant="contained" onClick={tentar}>
            Tentar de novo
          </Button>
        )}
        <Link component={RouterLink} to="/">
          Voltar ao início
        </Link>
      </Box>
      {import.meta.env.DEV && <DetalhesDoErro erro={erro} />}
    </Box>
  );
}

function DetalhesDoErro({ erro }: { erro: unknown }): ReactNode {
  const texto = erro instanceof Error ? (erro.stack ?? erro.message) : String(erro);
  return (
    <Alert severity="error" icon={false} sx={{ overflow: 'auto' }}>
      <details>
        <summary>Detalhes do erro</summary>
        <Box
          component="pre"
          sx={{ typography: 'caption', fontFamily: FONTE_MONO, whiteSpace: 'pre-wrap' }}
        >
          {texto}
        </Box>
      </details>
    </Alert>
  );
}
