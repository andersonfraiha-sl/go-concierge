import { Stack, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { Control } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import type { Valores } from './formularioDaFerramenta';

const METODOS = ['', 'GET', 'POST', 'PUT', 'PATCH', 'DELETE'];

type Props = { control: Control<Valores>; travado: boolean };

export function CamposDeConexao({ control, travado }: Props): ReactNode {
  return (
    <>
      <Stack component="section" aria-label="Conexão" spacing={1.5}>
        <Typography variant="subtitle2" component="h3">
          Conexão
        </Typography>
        <CampoTexto
          control={control}
          name="recurso.conexao.metodo"
          select
          label="Método"
          disabled={travado}
          slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
        >
          {METODOS.map((metodo) => (
            <option key={metodo} value={metodo}>
              {metodo || '—'}
            </option>
          ))}
        </CampoTexto>
        <CampoTexto
          control={control}
          name="recurso.conexao.url"
          label="Endereço"
          disabled={travado}
        />
        <CampoTexto
          control={control}
          name="recurso.conexao.corpo"
          label="Corpo da chamada"
          disabled={travado}
          multiline
          minRows={2}
        />
      </Stack>

      <CampoTexto
        control={control}
        name="textoAmostra"
        label="Amostra de retorno"
        disabled={travado}
        multiline
        minRows={4}
        slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
      />
    </>
  );
}
