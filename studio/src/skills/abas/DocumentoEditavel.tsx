import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { Alert, Box, Button, IconButton, Paper, TextField, Typography } from '@mui/material';
import { useState, type ReactNode } from 'react';
import { salvarArquivo, useArquivo } from '@/catalogo/contratos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';

type Props = {
  agente: string;
  caminho: string;
  titulo: string;
  editavel: boolean;
  onExcluir?: () => void;
};

export function DocumentoEditavel(props: Props): ReactNode {
  const { agente, caminho, titulo } = props;
  const arquivo = useArquivo(agente, caminho);

  if (arquivo.isPending) return <Typography variant="caption">Carregando {titulo}…</Typography>;
  if (arquivo.isError) {
    return (
      <Alert
        severity="error"
        action={
          <Button color="inherit" onClick={() => void arquivo.refetch()}>
            Tentar de novo
          </Button>
        }
      >
        Não foi possível abrir {titulo}.
      </Alert>
    );
  }
  return <Editor {...props} inicial={arquivo.data.conteudo} />;
}

/** Monta só depois da leitura: o texto nasce do conteúdo salvo e é o `valor` estável do autosave. */
function Editor({ agente, caminho, titulo, editavel, onExcluir, inicial }: Props & { inicial: string }): ReactNode {
  const [texto, setTexto] = useState(inicial);
  const { estado, tentarDeNovo } = useAutosave({
    valor: texto,
    salvar: (conteudo) => salvarArquivo({ agente, caminho, conteudo }),
  });

  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="subtitle2" sx={{ flex: 1 }}>
          {titulo}
        </Typography>
        <SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />
        {editavel && onExcluir && (
          <IconButton size="small" aria-label={`Excluir ${titulo}`} onClick={onExcluir}>
            <DeleteOutlinedIcon fontSize="small" />
          </IconButton>
        )}
      </Box>
      <TextField
        multiline
        minRows={4}
        fullWidth
        disabled={!editavel}
        value={texto}
        onChange={(e) => setTexto(e.target.value)}
        slotProps={{ htmlInput: { 'aria-label': titulo } }}
      />
    </Paper>
  );
}
