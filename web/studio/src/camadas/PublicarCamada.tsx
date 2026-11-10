import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useForm } from 'react-hook-form';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { CampoTexto } from '@/shared/ui/CampoTexto';

type Props = {
  desabilitado: boolean;
  /** Rejeita com o erro que a pessoa precisa ler; a mensagem aparece sob o campo. */
  onPublicar: (resumo: string) => Promise<void>;
};

/** Botão Publicar e o diálogo do resumo, opcional, que vai para o histórico. */
export function PublicarCamada({ desabilitado, onPublicar }: Props): ReactNode {
  const [aberto, setAberto] = useState(false);
  return (
    <>
      <Button variant="contained" disabled={desabilitado} onClick={() => setAberto(true)}>
        Publicar
      </Button>
      {aberto && <DialogoDePublicacao onPublicar={onPublicar} onFechar={() => setAberto(false)} />}
    </>
  );
}

type PropsDoDialogo = { onPublicar: Props['onPublicar']; onFechar: () => void };

function DialogoDePublicacao({ onPublicar, onFechar }: PropsDoDialogo): ReactNode {
  const { control, handleSubmit, setError, formState } = useForm({
    defaultValues: { resumo: '' },
  });

  const enviar = handleSubmit(async ({ resumo }) => {
    try {
      await onPublicar(resumo.trim());
      onFechar();
    } catch (causa) {
      setError('resumo', { message: mensagemDe(causa) });
    }
  });

  return (
    <Dialog open fullWidth maxWidth="xs" onClose={onFechar}>
      <form onSubmit={(evento) => void enviar(evento)}>
        <DialogTitle>Publicar sua versão</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          <DialogContentText>
            A versão publicada é a que o concierge passa a usar. O resumo é opcional e fica no
            histórico.
          </DialogContentText>
          <CampoTexto
            control={control}
            name="resumo"
            label="Resumo"
            autoFocus
            fullWidth
            multiline
            minRows={2}
            margin="dense"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={onFechar}>Cancelar</Button>
          <Button type="submit" variant="contained" disabled={formState.isSubmitting}>
            Publicar
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
