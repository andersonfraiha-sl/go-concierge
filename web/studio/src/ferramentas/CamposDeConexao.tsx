import { Stack, TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { type Cabecalho, METODOS } from './contratos';
import { rotuloDoCampo, type Valores } from './formularioDaFerramenta';
import { ListaEditavel } from './ListaEditavel';

type Props = { control: Control<Valores>; travado: boolean };

export function CamposDeConexao({ control, travado }: Props): ReactNode {
  return (
    <Stack component="section" aria-label="Conexão" spacing={1.5}>
      <CampoTexto
        control={control}
        name="recurso.conexao.metodo"
        select
        label="Método"
        disabled={travado}
        slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
      >
        {Object.entries(METODOS).map(([valor, rotulo]) => (
          <option key={valor} value={valor}>
            {rotulo}
          </option>
        ))}
      </CampoTexto>
      <CampoTexto
        control={control}
        name="recurso.conexao.url"
        label="Endereço"
        disabled={travado}
      />
      <Controller
        control={control}
        name="recurso.conexao.headers"
        render={({ field }) => (
          <ListaEditavel<Cabecalho>
            titulo="Cabeçalhos"
            adicionar="Adicionar cabeçalho"
            linhas={field.value}
            desabilitado={travado}
            criarLinha={() => ({ chave: '', valor: '' })}
            onMudar={field.onChange}
            renderLinha={(cabecalho, mudarLinha) => (
              <>
                <TextField
                  size="small"
                  placeholder="Authorization"
                  value={cabecalho.chave}
                  disabled={travado}
                  slotProps={rotuloDoCampo('Chave do cabeçalho')}
                  onChange={(e) => mudarLinha({ chave: e.target.value })}
                />
                <TextField
                  size="small"
                  placeholder="Bearer token"
                  value={cabecalho.valor}
                  disabled={travado}
                  slotProps={rotuloDoCampo('Valor do cabeçalho')}
                  onChange={(e) => mudarLinha({ valor: e.target.value })}
                />
              </>
            )}
          />
        )}
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
  );
}
