import { Box } from '@mui/material';
import { RAIOS, tokens } from '@/theme/tokens';
import type { Mensagem } from './CopilotoProvider';

type Props = { mensagem: Pick<Mensagem, 'papel' | 'texto' | 'tom'> };

// Bico da bolha do lado de quem fala: canto inferior direito da pessoa, esquerdo do Copiloto.
const BICO = 4;

export function BolhaDeMensagem({ mensagem }: Props) {
  const daPessoa = mensagem.papel === 'pessoa';
  const comErro = mensagem.tom === 'erro';
  return (
    <Box
      sx={{
        alignSelf: daPessoa ? 'flex-end' : 'flex-start',
        maxWidth: daPessoa ? '85%' : '92%',
        px: 2,
        py: 1.25,
        borderRadius: daPessoa
          ? `${RAIOS.painel}px ${RAIOS.painel}px ${BICO}px ${RAIOS.painel}px`
          : `${RAIOS.painel}px ${RAIOS.painel}px ${RAIOS.painel}px ${BICO}px`,
        bgcolor: daPessoa ? tokens.color.background.tint : 'background.paper',
        border: daPessoa ? 0 : 1,
        borderColor: comErro ? tokens.color.danger.main : tokens.color.border.default,
        color: comErro ? tokens.color.danger.main : 'text.primary',
        fontSize: 13,
        lineHeight: 1.55,
        whiteSpace: 'pre-wrap',
        overflowWrap: 'anywhere',
      }}
    >
      {mensagem.texto}
    </Box>
  );
}
