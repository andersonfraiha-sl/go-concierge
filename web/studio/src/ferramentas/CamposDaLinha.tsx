import { TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { rotuloDoCampo } from './formularioDaFerramenta';

type TextoProps = {
  rotulo: string;
  dica: string;
  valor: string;
  travado: boolean;
  larguraTotal?: boolean;
  onMudar: (texto: string) => void;
};

export function TextoDaLinha({
  rotulo,
  dica,
  valor,
  travado,
  larguraTotal,
  onMudar,
}: TextoProps): ReactNode {
  return (
    <TextField
      size="small"
      fullWidth={larguraTotal}
      placeholder={dica}
      value={valor}
      disabled={travado}
      slotProps={rotuloDoCampo(rotulo)}
      onChange={(evento) => onMudar(evento.target.value)}
    />
  );
}

type SeletorProps = {
  rotulo: string;
  valor: string;
  opcoes: readonly (readonly [string, string])[];
  travado: boolean;
  onMudar: (valor: string) => void;
};

export function SeletorDaLinha({
  rotulo,
  valor,
  opcoes,
  travado,
  onMudar,
}: SeletorProps): ReactNode {
  return (
    <TextField
      select
      size="small"
      value={valor}
      disabled={travado}
      slotProps={{ ...rotuloDoCampo(rotulo), select: { native: true } }}
      onChange={(evento) => onMudar(evento.target.value)}
    >
      {opcoes.map(([valorDaOpcao, rotuloDaOpcao]) => (
        <option key={valorDaOpcao} value={valorDaOpcao}>
          {rotuloDaOpcao}
        </option>
      ))}
    </TextField>
  );
}
