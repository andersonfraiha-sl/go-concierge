import { Box, Button, TextField, Typography } from '@mui/material';
import { type ReactNode, useRef, useState } from 'react';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  importarZip,
  mensagemDeErro,
  type NoDaArvore,
  urlExportar,
  useRecarregarAvancado,
} from './contratos';

type Props = { no: NoDaArvore; editar: boolean };

export function ImportarExportarAba({ no, editar }: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <SectionCard titulo="Exportar" ajuda={`Baixa ${no.caminho} com todos os arquivos, num zip.`}>
        <Box>
          <Button component="a" variant="outlined" href={urlExportar(no.caminho)}>
            Exportar zip
          </Button>
        </Box>
      </SectionCard>
      {editar && <Importar />}
    </Box>
  );
}

function Importar(): ReactNode {
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();
  const seletor = useRef<HTMLInputElement>(null);
  const [destino, setDestino] = useState('');
  const [zip, setZip] = useState<File | null>(null);
  const [confirmando, setConfirmando] = useState(false);

  const importar = async (): Promise<void> => {
    setConfirmando(false);
    if (!zip) return;
    try {
      const instalado = await importarZip(destino.trim(), zip);
      await recarregar();
      mostrar(`Importado em ${instalado.instalado}: ${instalado.arquivos.length} arquivos`);
      setZip(null);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  };

  return (
    <SectionCard
      titulo="Importar"
      ajuda="Instala um zip exportado por aqui. Arquivos com o mesmo nome são substituídos."
    >
      <TextField
        size="small"
        label="Instalar em"
        helperText="Pasta de destino, como aparece na lista."
        value={destino}
        onChange={(evento) => setDestino(evento.target.value)}
      />
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Button variant="outlined" onClick={() => seletor.current?.click()}>
          Escolher zip
        </Button>
        <Typography variant="body2" color="text.secondary">
          {zip?.name ?? 'Nenhum arquivo escolhido'}
        </Typography>
        <input
          ref={seletor}
          hidden
          type="file"
          accept=".zip,application/zip"
          aria-label="Arquivo zip"
          onChange={(evento) => setZip(evento.target.files?.[0] ?? null)}
        />
      </Box>
      <Box>
        <Button
          variant="contained"
          disabled={destino.trim() === '' || !zip}
          onClick={() => setConfirmando(true)}
        >
          Importar
        </Button>
      </Box>
      <ConfirmDialog
        aberto={confirmando}
        titulo="Importar este zip?"
        texto={`Os arquivos do zip entram em ${destino.trim()} e substituem os que têm o mesmo nome.`}
        confirmar="Importar"
        onConfirmar={() => void importar()}
        onCancelar={() => setConfirmando(false)}
      />
    </SectionCard>
  );
}
