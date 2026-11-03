import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button, Typography } from '@mui/material';
import { type ReactNode, useRef, useState } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
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

const ImportacaoSchema = z.object({
  destino: z.string().refine((destino) => destino.trim() !== ''),
  zip: z
    .instanceof(File)
    .nullable()
    .refine((zip) => zip !== null),
});

function Importar(): ReactNode {
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();
  const seletor = useRef<HTMLInputElement>(null);
  const [confirmando, setConfirmando] = useState(false);
  const { control, handleSubmit, setValue } = useForm({
    resolver: zodResolver(ImportacaoSchema),
    defaultValues: { destino: '', zip: null },
  });
  const [destino, zip] = useWatch({ control, name: ['destino', 'zip'] });

  const importar = handleSubmit(async ({ destino, zip }) => {
    setConfirmando(false);
    if (!zip) return;
    try {
      const instalado = await importarZip(destino.trim(), zip);
      await recarregar();
      mostrar(`Importado em ${instalado.instalado}: ${instalado.arquivos.length} arquivos`);
      setValue('zip', null);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  });

  return (
    <SectionCard
      titulo="Importar"
      ajuda="Instala um zip exportado por aqui. Arquivos com o mesmo nome são substituídos."
    >
      <CampoTexto
        control={control}
        name="destino"
        size="small"
        label="Instalar em"
        helperText="Pasta de destino, como aparece na lista."
      />
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Button variant="outlined" onClick={() => seletor.current?.click()}>
          Escolher zip
        </Button>
        <Typography variant="body2" color="text.secondary">
          {zip?.name ?? 'Nenhum arquivo escolhido'}
        </Typography>
        <Controller
          control={control}
          name="zip"
          render={({ field }) => (
            <input
              ref={seletor}
              hidden
              type="file"
              accept=".zip,application/zip"
              aria-label="Arquivo zip"
              onChange={(evento) => field.onChange(evento.target.files?.[0] ?? null)}
            />
          )}
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
