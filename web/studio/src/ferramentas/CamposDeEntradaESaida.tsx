import { Checkbox, FormControlLabel } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller } from 'react-hook-form';
import type { Ferramenta } from '@/catalogo/contratos';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SeletorDaLinha, TextoDaLinha } from './CamposDaLinha';
import type { Valores } from './formularioDaFerramenta';
import { ListaEditavel } from './ListaEditavel';

const TIPOS_DE_DADO = ['string', 'number', 'boolean', 'date', 'array', 'object'].map(
  (tipo) => [tipo, tipo] as const,
);

type Props = { control: Control<Valores>; travado: boolean };
type Parametro = Ferramenta['params'][number];
type CampoDeSaida = Ferramenta['saida'][number];

type LinhaDeParametroProps = {
  parametro: Parametro;
  travado: boolean;
  mudar: (parcial: Partial<Parametro>) => void;
};

function LinhaDeParametro({ parametro, travado, mudar }: LinhaDeParametroProps): ReactNode {
  return (
    <>
      <TextoDaLinha
        rotulo="Nome do parâmetro"
        dica="unidade_id"
        valor={parametro.name}
        travado={travado}
        onMudar={(name) => mudar({ name })}
      />
      <SeletorDaLinha
        rotulo="Tipo do parâmetro"
        valor={parametro.type ?? 'string'}
        opcoes={TIPOS_DE_DADO}
        travado={travado}
        onMudar={(type) => mudar({ type })}
      />
      <FormControlLabel
        label="Obrigatório"
        control={
          <Checkbox
            size="small"
            checked={parametro.required ?? false}
            disabled={travado}
            onChange={(evento) => mudar({ required: evento.target.checked })}
          />
        }
      />
      <TextoDaLinha
        rotulo="Descrição do parâmetro"
        dica="O que é, formato, exemplo"
        valor={parametro.description ?? ''}
        travado={travado}
        larguraTotal
        onMudar={(description) => mudar({ description })}
      />
    </>
  );
}

export function CamposDeParametros({ control, travado }: Props): ReactNode {
  return (
    <Controller
      control={control}
      name="recurso.params"
      render={({ field }) => (
        <ListaEditavel<Parametro>
          titulo="Parâmetros"
          adicionar="Adicionar parâmetro"
          linhas={field.value}
          desabilitado={travado}
          criarLinha={() => ({ name: '', type: 'string', required: false })}
          onMudar={field.onChange}
          renderLinha={(parametro, mudar) => (
            <LinhaDeParametro parametro={parametro} travado={travado} mudar={mudar} />
          )}
        />
      )}
    />
  );
}

type LinhaDeSaidaProps = {
  campo: CampoDeSaida;
  travado: boolean;
  mudar: (parcial: Partial<CampoDeSaida>) => void;
};

function LinhaDeSaida({ campo, travado, mudar }: LinhaDeSaidaProps): ReactNode {
  return (
    <>
      <TextoDaLinha
        rotulo="Nome do campo de saída"
        dica="itens"
        valor={campo.name}
        travado={travado}
        onMudar={(name) => mudar({ name })}
      />
      <SeletorDaLinha
        rotulo="Tipo do campo de saída"
        valor={campo.type ?? 'string'}
        opcoes={TIPOS_DE_DADO}
        travado={travado}
        onMudar={(type) => mudar({ type })}
      />
      <TextoDaLinha
        rotulo="Caminho do campo de saída"
        dica="data.items[].id"
        valor={campo.path ?? ''}
        travado={travado}
        onMudar={(path) => mudar({ path })}
      />
      <TextoDaLinha
        rotulo="Descrição do campo de saída"
        dica="O que o campo significa"
        valor={campo.description ?? ''}
        travado={travado}
        larguraTotal
        onMudar={(description) => mudar({ description })}
      />
    </>
  );
}

function ListaDeSaida({ control, travado }: Props): ReactNode {
  return (
    <Controller
      control={control}
      name="recurso.saida"
      render={({ field }) => (
        <ListaEditavel<CampoDeSaida>
          titulo="Saída"
          adicionar="Adicionar campo de saída"
          linhas={field.value}
          desabilitado={travado}
          criarLinha={() => ({ name: '' })}
          onMudar={field.onChange}
          renderLinha={(campo, mudar) => (
            <LinhaDeSaida campo={campo} travado={travado} mudar={mudar} />
          )}
        />
      )}
    />
  );
}

export function CamposDeSaida({ control, travado }: Props): ReactNode {
  return (
    <>
      <ListaDeSaida control={control} travado={travado} />
      <CampoTexto
        control={control}
        name="recurso.output"
        label="Resultado principal"
        disabled={travado}
      />
    </>
  );
}
