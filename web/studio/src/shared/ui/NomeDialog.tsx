import { zodResolver } from '@hookform/resolvers/zod';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from '@mui/material';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { paraIdentificador } from '@/skills/contratos';
import { CampoTexto } from './CampoTexto';

export type NomeEscolhido = { nome: string; titulo: string };

type Props = {
  titulo: string;
  tituloInicial: string;
  confirmar: string;
  /** Rejeita com Error cuja mensagem aparece sob o campo. */
  onEnviar: (escolhido: NomeEscolhido) => Promise<void>;
  onCancelar: () => void;
};

const NomeSchema = z.object({
  nome: z
    .string()
    .refine((nome) => paraIdentificador(nome) !== '', 'Use letras ou números no nome.'),
});

/** Monte só quando aberto: o campo nasce de `tituloInicial`. */
export function NomeDialog({
  titulo,
  tituloInicial,
  confirmar,
  onEnviar,
  onCancelar,
}: Props): ReactNode {
  const { control, handleSubmit, setError, formState } = useForm({
    resolver: zodResolver(NomeSchema),
    defaultValues: { nome: tituloInicial },
  });
  const identificador = paraIdentificador(useWatch({ control, name: 'nome' }));

  const enviar = handleSubmit(async ({ nome }) => {
    try {
      await onEnviar({ nome: paraIdentificador(nome), titulo: nome.trim() });
    } catch (causa) {
      setError('nome', { message: mensagemDe(causa) });
    }
  });

  return (
    <Dialog open fullWidth maxWidth="xs" onClose={onCancelar}>
      <form onSubmit={(e) => void enviar(e)}>
        <DialogTitle>{titulo}</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          <CampoTexto
            control={control}
            name="nome"
            autoFocus
            fullWidth
            margin="dense"
            label="Nome"
          />
          <Typography variant="caption" color="text.secondary">
            Identificador: {identificador}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={onCancelar}>Cancelar</Button>
          <Button
            type="submit"
            variant="contained"
            disabled={identificador === '' || formState.isSubmitting}
          >
            {confirmar}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
