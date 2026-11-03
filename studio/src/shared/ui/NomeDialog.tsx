import { Button, Dialog, DialogActions, DialogContent, DialogTitle, TextField, Typography } from '@mui/material';
import { useState, type FormEvent, type ReactNode } from 'react';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { paraIdentificador } from '@/skills/contratos';

export type NomeEscolhido = { nome: string; titulo: string };

type Props = {
  titulo: string;
  tituloInicial: string;
  confirmar: string;
  /** Rejeita com Error cuja mensagem aparece sob o campo. */
  onEnviar: (escolhido: NomeEscolhido) => Promise<void>;
  onCancelar: () => void;
};

/** Monte só quando aberto: o estado do campo nasce de `tituloInicial`. */
export function NomeDialog({ titulo, tituloInicial, confirmar, onEnviar, onCancelar }: Props): ReactNode {
  const [nome, setNome] = useState(tituloInicial);
  const [erro, setErro] = useState<string | null>(null);
  const [enviando, setEnviando] = useState(false);
  const identificador = paraIdentificador(nome);

  const enviar = async (e: FormEvent) => {
    e.preventDefault();
    if (identificador === '' || enviando) return;
    setEnviando(true);
    setErro(null);
    try {
      await onEnviar({ nome: identificador, titulo: nome.trim() });
    } catch (causa) {
      setErro(mensagemDe(causa));
    } finally {
      setEnviando(false);
    }
  };

  return (
    <Dialog open fullWidth maxWidth="xs" onClose={onCancelar}>
      <form onSubmit={(e) => void enviar(e)}>
        <DialogTitle>{titulo}</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          <TextField
            autoFocus
            fullWidth
            margin="dense"
            label="Nome"
            value={nome}
            onChange={(e) => setNome(e.target.value)}
            error={erro !== null}
            helperText={erro}
          />
          <Typography variant="caption" color="text.secondary">
            Identificador: {identificador}
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={onCancelar}>Cancelar</Button>
          <Button type="submit" variant="contained" disabled={identificador === '' || enviando}>
            {confirmar}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
