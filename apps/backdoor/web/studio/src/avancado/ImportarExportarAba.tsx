import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button, Typography } from '@mui/material';
import { type ReactNode, type RefObject, useRef, useState } from 'react';
import { type Control, Controller, type UseFormSetValue, useForm, useWatch } from 'react-hook-form';
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

type Importacao = z.input<typeof ImportacaoSchema>;

function useImportarZip(
  setValue: UseFormSetValue<Importacao>,
): (importacao: z.output<typeof ImportacaoSchema>) => Promise<void> {
  const { mostrar } = useSnackbar();
  const recarregar = useRecarregarAvancado();
  return async ({ destino, zip }) => {
    if (!zip) return;
    try {
      const instalado = await importarZip(destino.trim(), zip);
      await recarregar();
      mostrar(`Importado em ${instalado.instalado}: ${instalado.arquivos.length} arquivos`);
      setValue('zip', null);
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  };
}

function Importar(): ReactNode {
  const seletor = useRef<HTMLInputElement>(null);
  const [confirmando, setConfirmando] = useState(false);
  const { control, handleSubmit, setValue } = useForm({
    resolver: zodResolver(ImportacaoSchema),
    defaultValues: { destino: '', zip: null },
  });
  const [destino, zip] = useWatch({ control, name: ['destino', 'zip'] });
  const importar = useImportarZip(setValue);
  const confirmar = handleSubmit((importacao) => {
    setConfirmando(false);
    return importar(importacao);
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
      <EscolhaDoZip control={control} seletor={seletor} nome={zip?.name} />
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
        onConfirmar={() => void confirmar()}
        onCancelar={() => setConfirmando(false)}
      />
    </SectionCard>
  );
}

type EscolhaProps = {
  control: Control<Importacao>;
  seletor: RefObject<HTMLInputElement | null>;
  nome: string | undefined;
};

function EscolhaDoZip({ control, seletor, nome }: EscolhaProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Button variant="outlined" onClick={() => seletor.current?.click()}>
        Escolher zip
      </Button>
      <Typography variant="body2" color="textSecondary">
        {nome ?? 'Nenhum arquivo escolhido'}
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
  );
}
