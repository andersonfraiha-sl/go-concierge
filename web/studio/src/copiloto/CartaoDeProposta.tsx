import { Box, Button, Typography } from '@mui/material';
import { type ReactNode, useId, useState } from 'react';
import { useEu } from '@/acesso/eu';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { RAIOS, tokens } from '@/theme/tokens';
import { podeAplicar } from './aplicar';
import { useCopiloto } from './CopilotoProvider';
import type { Proposta } from './contratos';
import { MENSAGEM_DE_ESPERA } from './useAplicarPropostas';

type Props = { proposta: Proposta };

type Situacao = { tipo: 'pronta' } | { tipo: 'erro'; mensagem: string };
type Confirmacao = { texto: string; responder: (confirmou: boolean) => void };

const PRONTA: Situacao = { tipo: 'pronta' };
const linha = `1px solid ${tokens.color.border.subtle}`;

export function CartaoDeProposta({ proposta }: Props) {
  const { aplicar, aplicadas, emCurso, descartar } = useCopiloto();
  const { data: eu } = useEu();
  const [situacao, setSituacao] = useState<Situacao>(PRONTA);
  const [confirmacao, setConfirmacao] = useState<Confirmacao | null>(null);
  const idDoResumo = useId();

  const confirmar = (texto: string) =>
    new Promise<boolean>((responder) => setConfirmacao({ texto, responder }));

  const responder = (confirmou: boolean) => {
    confirmacao?.responder(confirmou);
    setConfirmacao(null);
  };

  const aoAplicar = async (escopo: string | null) => {
    setSituacao(PRONTA);
    try {
      await aplicar(proposta, { escopo, confirmar });
      setSituacao(PRONTA);
    } catch (erro) {
      setSituacao({ tipo: 'erro', mensagem: mensagemDe(erro) });
    }
  };

  const descartarEsta = () => descartar(proposta.id);

  const rodape = (): ReactNode => {
    if (aplicadas.has(proposta.id)) {
      return (
        <Aviso cor={tokens.color.success.main} fundo={tokens.color.success.background}>
          Aplicada
        </Aviso>
      );
    }
    if (eu && !podeAplicar(eu, proposta.operacao)) {
      return (
        <Acoes>
          <Typography sx={{ flex: 1, fontSize: 13, color: 'text.secondary' }}>
            Você não pode aplicar esta.
          </Typography>
          <BotaoDescartar onClick={descartarEsta} />
        </Acoes>
      );
    }
    return (
      <>
        {situacao.tipo === 'erro' && (
          <Aviso cor={tokens.color.danger.main} papel="alert">
            {situacao.mensagem}
          </Aviso>
        )}
        <Acoes>
          <Button
            variant="contained"
            size="small"
            fullWidth
            disabled={!eu || emCurso !== null}
            onClick={() => eu && void aoAplicar(eu.administradoraId)}
          >
            {rotuloDoAplicar(emCurso, proposta.id)}
          </Button>
          <BotaoDescartar onClick={descartarEsta} />
        </Acoes>
      </>
    );
  };

  return (
    <Box
      role="group"
      aria-labelledby={idDoResumo}
      sx={{
        alignSelf: 'stretch',
        bgcolor: 'background.paper',
        border: 1,
        borderColor: tokens.color.border.strong,
        borderRadius: `${RAIOS.painel}px`,
        overflow: 'hidden',
      }}
    >
      <Box
        id={idDoResumo}
        sx={{
          px: 2,
          py: 1.25,
          bgcolor: tokens.color.background.subtle,
          borderBottom: linha,
          fontSize: 13,
          fontWeight: 600,
          color: tokens.color.text.strong,
        }}
      >
        {proposta.resumo}
      </Box>
      {rodape()}
      {confirmacao && (
        <ConfirmDialog
          aberto
          titulo="Excluir skill"
          texto={confirmacao.texto}
          confirmar="Excluir"
          onConfirmar={() => responder(true)}
          onCancelar={() => responder(false)}
        />
      )}
    </Box>
  );
}

function rotuloDoAplicar(emCurso: string | null, id: string): string {
  if (emCurso === null) return 'Aplicar';
  return emCurso === id ? 'Aplicando…' : MENSAGEM_DE_ESPERA;
}

function Acoes({ children }: { children: ReactNode }) {
  return (
    <Box sx={{ px: 2, py: 1.25, display: 'flex', alignItems: 'center', gap: 1 }}>{children}</Box>
  );
}

type AvisoProps = { cor: string; fundo?: string; papel?: 'alert'; children: ReactNode };

function Aviso({ cor, fundo, papel, children }: AvisoProps) {
  return (
    <Typography
      role={papel}
      sx={{
        px: 2,
        py: 1.25,
        whiteSpace: 'pre-line',
        fontSize: 13,
        fontWeight: 500,
        color: cor,
        bgcolor: fundo,
      }}
    >
      {children}
    </Typography>
  );
}

function BotaoDescartar({ onClick }: { onClick: () => void }) {
  return (
    <Button
      variant="outlined"
      color="inherit"
      size="small"
      fullWidth
      onClick={onClick}
      sx={{ color: 'text.secondary', borderColor: tokens.color.border.default }}
    >
      Descartar
    </Button>
  );
}
