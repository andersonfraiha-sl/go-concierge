import { zodResolver } from '@hookform/resolvers/zod';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  MenuItem,
  TextField,
} from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller, useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import {
  caminhoNaPasta,
  PASTA_INICIAL,
  PASTAS_DE_ARQUIVOS,
  type PastaDeArquivos,
  SUGESTAO_DE_NOME,
} from './contratos';

const NovoArquivoSchema = z.object({
  pasta: z.enum(PASTAS_DE_ARQUIVOS),
  nome: z.string().refine((nome) => nome.trim() !== ''),
});

type NovoProps = {
  aberto: boolean;
  onCriar: (caminho: string) => Promise<void>;
  onFechar: () => void;
};

type Novo = z.infer<typeof NovoArquivoSchema>;

export function NovoArquivoDialog({ aberto, onCriar, onFechar }: NovoProps): ReactNode {
  const { control, handleSubmit, setValue } = useForm({
    resolver: zodResolver(NovoArquivoSchema),
    defaultValues: { pasta: PASTA_INICIAL, nome: SUGESTAO_DE_NOME[PASTA_INICIAL] },
  });
  const nome = useWatch({ control, name: 'nome' });
  const criar = handleSubmit(({ pasta, nome: digitado }) =>
    onCriar(caminhoNaPasta(pasta, digitado.trim())),
  );

  const escolherPasta = (valor: string, mudarPasta: (pasta: PastaDeArquivos) => void): void => {
    const escolhida = PASTAS_DE_ARQUIVOS.find((pasta) => pasta === valor);
    if (!escolhida) return;
    mudarPasta(escolhida);
    setValue('nome', SUGESTAO_DE_NOME[escolhida]);
  };

  return (
    <Dialog open={aberto} onClose={onFechar} fullWidth maxWidth="xs">
      <DialogTitle>Novo arquivo</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
        <SeletorDePasta control={control} onEscolher={escolherPasta} />
        <CampoTexto control={control} name="nome" size="small" label="Nome do arquivo" />
      </DialogContent>
      <DialogActions>
        <Button onClick={onFechar}>Cancelar</Button>
        <Button variant="contained" disabled={nome.trim() === ''} onClick={() => void criar()}>
          Criar
        </Button>
      </DialogActions>
    </Dialog>
  );
}

type SeletorProps = {
  control: Control<Novo>;
  onEscolher: (valor: string, mudarPasta: (pasta: PastaDeArquivos) => void) => void;
};

function SeletorDePasta({ control, onEscolher }: SeletorProps): ReactNode {
  return (
    <Controller
      control={control}
      name="pasta"
      render={({ field }) => (
        <TextField
          select
          size="small"
          label="Pasta"
          value={field.value}
          onChange={(evento) => onEscolher(evento.target.value, field.onChange)}
        >
          {PASTAS_DE_ARQUIVOS.map((pasta) => (
            <MenuItem key={pasta} value={pasta}>
              {pasta}
            </MenuItem>
          ))}
        </TextField>
      )}
    />
  );
}
