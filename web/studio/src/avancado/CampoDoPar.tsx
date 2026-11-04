import { TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { rotuloDoCampo } from '@/ferramentas/formularioDaFerramenta';
import { type ParDoPayload, TIPOS, tipoDe, tipoInferido } from './payloadLivre';

type Props = {
  par: ParDoPayload;
  travado: boolean;
  erroChave?: string | undefined;
  erroValor?: string | undefined;
  onMudar: (parcial: Partial<ParDoPayload>) => void;
};

export function CampoDoPar({ par, travado, erroChave, erroValor, onMudar }: Props): ReactNode {
  // Linha nova acompanha o que se digita até a pessoa escolher o tipo; linha gravada mantém o dela.
  const digitar = (valor: string) =>
    onMudar(par.fixo ? { valor } : { valor, tipo: tipoInferido(valor) });
  return (
    <>
      <TextField
        size="small"
        placeholder="limite"
        value={par.chave}
        disabled={travado}
        error={erroChave !== undefined}
        helperText={erroChave}
        slotProps={rotuloDoCampo('Chave')}
        onChange={(e) => onMudar({ chave: e.target.value })}
      />
      <TextField
        size="small"
        placeholder="3"
        value={par.valor}
        disabled={travado}
        error={erroValor !== undefined}
        helperText={erroValor}
        slotProps={rotuloDoCampo('Valor')}
        onChange={(e) => digitar(e.target.value)}
      />
      <TextField
        select
        size="small"
        value={par.tipo}
        disabled={travado}
        slotProps={{ select: { native: true }, htmlInput: { 'aria-label': 'Tipo do valor' } }}
        onChange={(e) => onMudar({ tipo: tipoDe(e.target.value), fixo: true })}
      >
        {TIPOS.map((tipo) => (
          <option key={tipo} value={tipo}>
            {tipo}
          </option>
        ))}
      </TextField>
    </>
  );
}
