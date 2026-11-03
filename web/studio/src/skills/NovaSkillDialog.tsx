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
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { LIMITE_DO_RESUMO, paraIdentificador } from './contratos';

export type SkillNova = { nome: string; titulo: string; resumo: string; quandoUsar: string };

type Props = {
  /** Rejeita com Error cuja mensagem aparece sob o campo Nome. */
  onEnviar: (nova: SkillNova) => Promise<void>;
  onCancelar: () => void;
};

const NovaSkillSchema = z.object({
  titulo: z
    .string()
    .refine((titulo) => paraIdentificador(titulo) !== '', 'Use letras ou números no nome.'),
  resumo: z.string().max(LIMITE_DO_RESUMO),
  quandoUsar: z.string(),
});

/** Diálogo de criação: nome, resumo e critério de reconhecimento, enviados juntos. */
export function NovaSkillDialog({ onEnviar, onCancelar }: Props): ReactNode {
  const { control, handleSubmit, setError, formState } = useForm({
    resolver: zodResolver(NovaSkillSchema),
    defaultValues: { titulo: '', resumo: '', quandoUsar: '' },
  });
  const identificador = paraIdentificador(useWatch({ control, name: 'titulo' }));
  const resumo = useWatch({ control, name: 'resumo' });

  const enviar = handleSubmit(async ({ titulo, resumo: resumoEnviado, quandoUsar }) => {
    try {
      await onEnviar({
        nome: paraIdentificador(titulo),
        titulo: titulo.trim(),
        resumo: resumoEnviado,
        quandoUsar,
      });
    } catch (causa) {
      setError('titulo', { message: mensagemDe(causa) });
    }
  });

  return (
    <Dialog open fullWidth maxWidth="xs" onClose={onCancelar}>
      <form onSubmit={(e) => void enviar(e)}>
        <DialogTitle>Nova skill</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          <CampoTexto
            control={control}
            name="titulo"
            autoFocus
            fullWidth
            margin="dense"
            label="Nome"
          />
          <Typography variant="caption" color="text.secondary">
            Identificador: {identificador}
          </Typography>
          <CampoTexto
            control={control}
            name="resumo"
            fullWidth
            margin="dense"
            label="Resumo"
            helperText={`${resumo.length}/${LIMITE_DO_RESUMO}`}
            slotProps={{ htmlInput: { maxLength: LIMITE_DO_RESUMO } }}
          />
          <CampoTexto
            control={control}
            name="quandoUsar"
            multiline
            minRows={2}
            fullWidth
            margin="dense"
            label="Como reconhecer o pedido"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={onCancelar}>Cancelar</Button>
          <Button
            type="submit"
            variant="contained"
            disabled={identificador === '' || formState.isSubmitting}
          >
            Criar
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
