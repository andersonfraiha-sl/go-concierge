import { Stack } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { TextoDaLinha } from './CamposDaLinha';
import { type Cabecalho, METODOS } from './contratos';
import type { Valores } from './formularioDaFerramenta';
import { ListaEditavel } from './ListaEditavel';

type Props = { control: Control<Valores>; travado: boolean };

const novoCabecalho = (): Cabecalho => ({ chave: '', valor: '' });

type LinhaProps = {
  cabecalho: Cabecalho;
  travado: boolean;
  mudar: (parcial: Partial<Cabecalho>) => void;
};

function LinhaDeCabecalho({ cabecalho, travado, mudar }: LinhaProps): ReactNode {
  return (
    <>
      <TextoDaLinha
        rotulo="Chave do cabeçalho"
        dica="Authorization"
        valor={cabecalho.chave}
        travado={travado}
        onMudar={(chave) => mudar({ chave })}
      />
      <TextoDaLinha
        rotulo="Valor do cabeçalho"
        dica="Bearer token"
        valor={cabecalho.valor}
        travado={travado}
        onMudar={(valor) => mudar({ valor })}
      />
    </>
  );
}

function Cabecalhos({ control, travado }: Props): ReactNode {
  return (
    <Controller
      control={control}
      name="recurso.conexao.headers"
      render={({ field }) => (
        <ListaEditavel<Cabecalho>
          titulo="Cabeçalhos"
          adicionar="Adicionar cabeçalho"
          linhas={field.value}
          desabilitado={travado}
          criarLinha={novoCabecalho}
          onMudar={field.onChange}
          renderLinha={(cabecalho, mudar) => (
            <LinhaDeCabecalho cabecalho={cabecalho} travado={travado} mudar={mudar} />
          )}
        />
      )}
    />
  );
}

function Metodo({ control, travado }: Props): ReactNode {
  return (
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
  );
}

export function CamposDeConexao({ control, travado }: Props): ReactNode {
  return (
    <Stack component="section" aria-label="Conexão" spacing={1.5}>
      <Metodo control={control} travado={travado} />
      <CampoTexto
        control={control}
        name="recurso.conexao.url"
        label="Endereço"
        disabled={travado}
      />
      <Cabecalhos control={control} travado={travado} />
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
