import { TextField } from '@mui/material';
import type { ReactNode } from 'react';

type TextoProps = {
  rotulo: string;
  valor: string;
  editavel: boolean;
  linhas?: number;
  onMudar: (valor: string) => void;
};

// por que: um TextField declarativo; separar só espalharia as props.
export function Texto({ rotulo, valor, editavel, linhas, onMudar }: TextoProps): ReactNode {
  return (
    <TextField
      size="small"
      fullWidth
      label={rotulo}
      value={valor}
      disabled={!editavel}
      multiline={linhas !== undefined}
      minRows={linhas}
      onChange={(evento) => onMudar(evento.target.value)}
    />
  );
}
