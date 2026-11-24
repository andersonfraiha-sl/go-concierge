import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material';
import type { ReactNode } from 'react';

type Props = {
  aberto: boolean;
  titulo: string;
  texto: string;
  confirmar: string;
  onConfirmar: () => void;
  onCancelar: () => void;
};

// por que: só JSX declarativo, sem lógica; quebrar espalharia o markup sem ganho de leitura.
export function ConfirmDialog({
  aberto,
  titulo,
  texto,
  confirmar,
  onConfirmar,
  onCancelar,
}: Props): ReactNode {
  return (
    <Dialog open={aberto} onClose={onCancelar}>
      <DialogTitle>{titulo}</DialogTitle>
      <DialogContent>
        <DialogContentText>{texto}</DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancelar}>Cancelar</Button>
        <Button color="error" variant="contained" onClick={onConfirmar}>
          {confirmar}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
