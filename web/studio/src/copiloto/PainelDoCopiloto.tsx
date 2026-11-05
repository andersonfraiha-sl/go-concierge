import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import CloseRounded from '@mui/icons-material/CloseRounded';
import { Alert, Box, IconButton, Typography } from '@mui/material';
import { Component, type ReactNode } from 'react';
import { ICONES, PESOS, RAIOS, TIPOS, tokens } from '@/theme/tokens';
import { BolhaDeMensagem } from './BolhaDeMensagem';
import { CartaoDeProposta } from './CartaoDeProposta';
import { Compositor } from './Compositor';
import { type Mensagem, type Status, useCopiloto } from './CopilotoProvider';
import { EstadoVazio } from './EstadoVazio';

const AVISO_INDISPONIVEL = 'O Copiloto está fora do ar. O resto do Studio continua funcionando.';
const AVISO_DE_FALHA = 'O Copiloto teve um problema. Feche e abra de novo.';

const ROTULO_DO_STATUS: Record<Status, { texto: string; cor: string; corDoPonto: string }> = {
  pronto: {
    texto: 'Pronto',
    cor: 'success.onSoft',
    corDoPonto: 'success.main',
  },
  pensando: {
    texto: 'Pensando…',
    cor: 'text.muted',
    corDoPonto: 'text.muted',
  },
  indisponivel: {
    texto: 'Indisponível',
    cor: 'warning.onSoft',
    corDoPonto: 'warning.onSoft',
  },
};

const ESTILO_DO_PAINEL = {
  width: tokens.layout.copiloto,
  flexShrink: 0,
  // A moldura não rola: o painel ocupa a altura toda abaixo do topo, seja barra ou cabeçalho.
  alignSelf: 'stretch',
  minHeight: 0,
  display: 'flex',
  flexDirection: 'column',
  bgcolor: 'background.paper',
  borderRight: 1,
  borderColor: 'border.default',
} as const;

/** Painel à esquerda do conteúdo; um erro dentro dele nunca derruba o resto do Studio. */
export function PainelDoCopiloto() {
  const { aberto } = useCopiloto();
  if (!aberto) return null;
  return (
    <Box component="aside" aria-label="Copiloto" sx={ESTILO_DO_PAINEL}>
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
      <ListaDeMensagens mensagens={mensagens} />
      {status === 'indisponivel' && (
        <Alert severity="warning" sx={{ mx: 3, mt: 2, borderRadius: `${RAIOS.campo}px` }}>
          {AVISO_INDISPONIVEL}
        </Alert>
      )}
      <Compositor desabilitado={status !== 'pronto'} aoEnviar={enviar} />
    </>
  );
}

function ListaDeMensagens({ mensagens }: { mensagens: Mensagem[] }) {
  return (
    // column-reverse mantém a rolagem presa na última mensagem sem JS.
    <Box
      sx={{
        flex: 1,
        minHeight: 0,
        overflowY: 'auto',
        display: 'flex',
        flexDirection: 'column-reverse',
        bgcolor: 'background.subtle',
      }}
    >
      <Box sx={{ p: 3, display: 'flex', flexDirection: 'column', gap: 2 }}>
        {mensagens.length === 0 && <EstadoVazio />}
        {mensagens.map((mensagem) => (
          <MensagemComPropostas key={mensagem.id} mensagem={mensagem} />
        ))}
      </Box>
    </Box>
  );
}

function MensagemComPropostas({ mensagem }: { mensagem: Mensagem }) {
  return (
    <>
      <BolhaDeMensagem mensagem={mensagem} />
      {mensagem.propostas.map((proposta) => (
        <CartaoDeProposta key={proposta.id} proposta={proposta} />
      ))}
    </>
  );
}

const ESTILO_DO_CABECALHO = {
  height: 50,
  flexShrink: 0,
  px: 3,
  display: 'flex',
  alignItems: 'center',
  gap: 1.25,
  borderBottom: 1,
  borderColor: 'border.subtle',
} as const;

function Cabecalho({ status }: { status: Status }) {
  return (
    <Box sx={ESTILO_DO_CABECALHO}>
      <IconeDoCopiloto />
      <Typography sx={{ fontWeight: PESOS.semi, fontSize: TIPOS.body1, color: 'secondary.main' }}>
        Copiloto
      </Typography>
      <IndicadorDeStatus status={status} />
      <BotaoFechar />
    </Box>
  );
}

function IconeDoCopiloto() {
  return (
    <Box
      sx={{
        width: 26,
        height: 26,
        borderRadius: `${RAIOS.campo}px`,
        bgcolor: 'secondary.main',
        color: 'primary.contrastText',
        display: 'grid',
        placeItems: 'center',
      }}
    >
      <AutoAwesomeOutlined sx={{ fontSize: ICONES.pequeno }} />
    </Box>
  );
}

function IndicadorDeStatus({ status }: { status: Status }) {
  const rotulo = ROTULO_DO_STATUS[status];
  return (
    <Box
      role="status"
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 0.625,
        fontSize: TIPOS.caption,
        color: rotulo.cor,
      }}
    >
      <Box sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: rotulo.corDoPonto }} />
      {rotulo.texto}
    </Box>
  );
}

function BotaoFechar() {
  const { fechar } = useCopiloto();
  return (
    <IconButton
      aria-label="Fechar copiloto"
      onClick={fechar}
      sx={{
        ml: 'auto',
        width: 28,
        height: 28,
        borderRadius: `${RAIOS.campo - 1}px`,
        bgcolor: 'background.neutralFill',
        color: 'text.secondary',
      }}
    >
      <CloseRounded sx={{ fontSize: ICONES.minimo }} />
    </IconButton>
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
      <Typography variant="body2" sx={{ p: 3, color: 'error.main' }}>
        {AVISO_DE_FALHA}
      </Typography>
    );
  }
}
