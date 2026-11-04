import { TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { rotuloDoCampo } from '@/ferramentas/formularioDaFerramenta';
import type { ParDoPayload } from './payloadLivre';

type Props = {
  par: ParDoPayload;
  travado: boolean;
  onMudar: (parcial: Partial<ParDoPayload>) => void;
};

export function CampoDoPar({ par, travado, onMudar }: Props): ReactNode {
  return (
    <>
      <TextField
        size="small"
        placeholder="limite"
        value={par.chave}
        disabled={travado}
        slotProps={rotuloDoCampo('Chave')}
        onChange={(e) => onMudar({ chave: e.target.value })}
      />
      <TextField
        size="small"
        placeholder="3"
        value={par.valor}
        disabled={travado}
        slotProps={rotuloDoCampo('Valor')}
        onChange={(e) => onMudar({ valor: e.target.value })}
      />
    </>
  );
}
