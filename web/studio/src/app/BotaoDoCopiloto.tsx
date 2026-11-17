import { Button, type SxProps, type Theme } from '@mui/material';
import type { ReactNode } from 'react';
import { useCopiloto } from '@/copiloto/CopilotoProvider';
import { ICONES, PESOS, RAIO, SHELL, TIPOS } from '@/theme/tokens';
import { IconeDeLinha } from './iconesDoShell';

/** Na barra global o botão é mais baixo que no cabeçalho de detalhe. */
type Variante = 'barra' | 'detalhe';

const ALTURA: Record<Variante, number> = {
  barra: SHELL.alturaDoControle,
  detalhe: SHELL.alturaDoBotaoDoDetalhe,
};

const estiloDoBotao = (aberto: boolean, variante: Variante): SxProps<Theme> => ({
  height: ALTURA[variante],
  // Como no desenho (HTML sem reset), altura e largura não contam padding nem borda.
  boxSizing: 'content-box',
  px: 1.75,
  py: 0,
  minWidth: 0,
  gap: 1,
  flexShrink: 0,
  borderRadius: RAIO.chip,
  fontSize: TIPOS.body2,
  fontWeight: PESOS.semi,
  lineHeight: 'normal',
  border: 1,
  borderColor: aberto ? 'secondary.main' : 'border.strong',
  bgcolor: aberto ? 'secondary.main' : 'background.paper',
  color: aberto ? 'primary.contrastText' : 'secondary.main',
  '&:hover': { bgcolor: aberto ? 'secondary.main' : 'background.subtle' },
});

export function BotaoDoCopiloto({ variante }: { variante: Variante }): ReactNode {
  const { aberto, alternar, botaoDoCopiloto } = useCopiloto();
  return (
    <Button
      ref={botaoDoCopiloto}
      aria-pressed={aberto}
      onClick={alternar}
      sx={estiloDoBotao(aberto, variante)}
    >
      <IconeDeLinha nome="copiloto" tamanho={ICONES.medio} />
      Copiloto
    </Button>
  );
}
