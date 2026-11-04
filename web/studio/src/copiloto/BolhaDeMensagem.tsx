import { Box } from '@mui/material';
import { RAIOS, TIPOS } from '@/theme/tokens';
import type { Mensagem } from './CopilotoProvider';

type Props = { mensagem: Pick<Mensagem, 'papel' | 'texto' | 'tom'> };

// Bico da bolha do lado de quem fala: canto inferior direito da pessoa, esquerdo do Copiloto.
const BICO = 4;

const raioDaBolha = (daPessoa: boolean): string => {
  const { painel } = RAIOS;
  return daPessoa
    ? `${painel}px ${painel}px ${BICO}px ${painel}px`
    : `${painel}px ${painel}px ${painel}px ${BICO}px`;
};

export function BolhaDeMensagem({ mensagem }: Props) {
  const daPessoa = mensagem.papel === 'pessoa';
  const corDoErro = mensagem.tom === 'erro' ? 'error.main' : undefined;
  return (
    <Box
      sx={{
        alignSelf: daPessoa ? 'flex-end' : 'flex-start',
        maxWidth: daPessoa ? '85%' : '92%',
        px: 2,
        py: 1.25,
        borderRadius: raioDaBolha(daPessoa),
        bgcolor: daPessoa ? 'background.tint' : 'background.paper',
        border: daPessoa ? 0 : 1,
        borderColor: corDoErro ?? 'border.default',
        color: corDoErro ?? 'text.primary',
        fontSize: TIPOS.body2,
        lineHeight: 1.55,
        whiteSpace: 'pre-wrap',
        overflowWrap: 'anywhere',
      }}
    >
      {mensagem.texto}
    </Box>
  );
}
