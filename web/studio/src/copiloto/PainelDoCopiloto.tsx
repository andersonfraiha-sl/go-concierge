import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import CloseRounded from '@mui/icons-material/CloseRounded';
import { Alert, Box, IconButton, Typography } from '@mui/material';
import { Component, Fragment, type ReactNode } from 'react';
import { RAIOS, tokens } from '@/theme/tokens';
import { BolhaDeMensagem } from './BolhaDeMensagem';
import { CartaoDeProposta } from './CartaoDeProposta';
import { Compositor } from './Compositor';
import { type Status, useCopiloto } from './CopilotoProvider';
import { EstadoVazio } from './EstadoVazio';

const AVISO_INDISPONIVEL = 'O Copiloto está fora do ar. O resto do Studio continua funcionando.';
const AVISO_DE_FALHA = 'O Copiloto teve um problema. Feche e abra de novo.';

const ROTULO_DO_STATUS: Record<Status, { texto: string; cor: string }> = {
  pronto: { texto: 'Pronto', cor: tokens.color.success.main },
  pensando: { texto: 'Pensando…', cor: tokens.color.text.muted },
  indisponivel: { texto: 'Indisponível', cor: tokens.color.warning.text },
};

/** Painel à esquerda do conteúdo; um erro dentro dele nunca derruba o resto do Studio. */
export function PainelDoCopiloto() {
  const { aberto } = useCopiloto();
  if (!aberto) return null;
  return (
    <Box
      component="aside"
      aria-label="Copiloto"
      sx={{
        width: tokens.layout.copiloto,
        flexShrink: 0,
        position: 'sticky',
        top: 0,
        alignSelf: 'flex-start',
        height: `calc(100vh - ${tokens.layout.navbarHeight}px)`,
        display: 'flex',
        flexDirection: 'column',
        bgcolor: 'background.paper',
        borderRight: 1,
        borderColor: tokens.color.border.default,
      }}
    >
      <LimiteDeErro>
        <ConteudoDoPainel />
      </LimiteDeErro>
    </Box>
  );
}

function ConteudoDoPainel() {
  const { status, mensagens, enviar } = useCopiloto();
  return (
    <>
      <Cabecalho status={status} />
      {/* column-reverse mantém a rolagem presa na última mensagem sem JS. */}
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          overflowY: 'auto',
          display: 'flex',
          flexDirection: 'column-reverse',
          bgcolor: tokens.color.background.subtle,
        }}
      >
        <Box sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}>
          {mensagens.length === 0 && <EstadoVazio />}
          {mensagens.map((mensagem) => (
            <Fragment key={mensagem.id}>
              <BolhaDeMensagem mensagem={mensagem} />
              {mensagem.propostas.map((proposta) => (
                <CartaoDeProposta key={proposta.id} proposta={proposta} />
              ))}
            </Fragment>
          ))}
        </Box>
      </Box>
      {status === 'indisponivel' && (
        <Alert severity="warning" sx={{ mx: 3, mt: 2, borderRadius: `${RAIOS.campo}px` }}>
          {AVISO_INDISPONIVEL}
        </Alert>
      )}
      <Compositor desabilitado={status !== 'pronto'} aoEnviar={enviar} />
    </>
  );
}

function Cabecalho({ status }: { status: Status }) {
  const { alternar } = useCopiloto();
  const rotulo = ROTULO_DO_STATUS[status];
  return (
    <Box
      sx={{
        height: 50,
        flexShrink: 0,
        px: 3,
        display: 'flex',
        alignItems: 'center',
        gap: 1.25,
        borderBottom: `1px solid ${tokens.color.border.subtle}`,
      }}
    >
      <Box
        sx={{
          width: 26,
          height: 26,
          borderRadius: `${RAIOS.campo}px`,
          bgcolor: tokens.color.navy,
          color: 'primary.contrastText',
          display: 'grid',
          placeItems: 'center',
        }}
      >
        <AutoAwesomeOutlined sx={{ fontSize: 14 }} />
      </Box>
      <Typography sx={{ fontWeight: 600, fontSize: 14, color: tokens.color.navy }}>
        Copiloto
      </Typography>
      <Box
        role="status"
        sx={{ display: 'flex', alignItems: 'center', gap: 0.625, fontSize: 12, color: rotulo.cor }}
      >
        <Box sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: rotulo.cor }} />
        {rotulo.texto}
      </Box>
      <IconButton
        aria-label="Fechar copiloto"
        onClick={alternar}
        size="small"
        sx={{ ml: 'auto', borderRadius: `${RAIOS.campo}px`, color: 'text.secondary' }}
      >
        <CloseRounded sx={{ fontSize: 16 }} />
      </IconButton>
    </Box>
  );
}

type EstadoDoLimite = { quebrou: boolean };

// React só captura erro de render com componente de classe; não há hook equivalente.
class LimiteDeErro extends Component<{ children: ReactNode }, EstadoDoLimite> {
  state: EstadoDoLimite = { quebrou: false };

  static getDerivedStateFromError(): EstadoDoLimite {
    return { quebrou: true };
  }

  componentDidCatch(erro: unknown): void {
    console.error('painel do Copiloto:', erro);
  }

  render(): ReactNode {
    if (!this.state.quebrou) return this.props.children;
    return (
      <Typography variant="body2" sx={{ p: 3, color: tokens.color.danger.main }}>
        {AVISO_DE_FALHA}
      </Typography>
    );
  }
}
