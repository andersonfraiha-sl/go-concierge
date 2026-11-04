import type { ReactNode } from 'react';
import type { Control } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { CamposDeAjustes } from './CamposDeAjustes';
import { CamposDeConexao } from './CamposDeConexao';
import { CamposDeParametros, CamposDeSaida } from './CamposDeEntradaESaida';
import { CamposDeIdentificacao } from './CamposDeIdentificacao';
import type { Valores } from './formularioDaFerramenta';

export const SECOES = [
  'Básico',
  'Parâmetros',
  'Saída',
  'Conexão',
  'Amostra',
  'Ajustes por condomínio',
] as const;
export type Secao = (typeof SECOES)[number];

type Campos = { control: Control<Valores>; travado: boolean };

function AmostraDeRetorno({ control, travado }: Campos): ReactNode {
  return (
    <CampoTexto
      control={control}
      name="textoAmostra"
      label="Amostra de retorno"
      disabled={travado}
      multiline
      minRows={4}
      slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
    />
  );
}

/** Conteúdo de cada seção do drawer; a ordem aqui é a ordem na tela. */
export function secoesDoFormulario(
  control: Control<Valores>,
  travado: boolean,
  tipoInicial: string,
): { titulo: Secao; conteudo: ReactNode }[] {
  const campos = { control, travado };
  return [
    { titulo: 'Básico', conteudo: <CamposDeIdentificacao {...campos} tipoInicial={tipoInicial} /> },
    { titulo: 'Parâmetros', conteudo: <CamposDeParametros {...campos} /> },
    { titulo: 'Saída', conteudo: <CamposDeSaida {...campos} /> },
    { titulo: 'Conexão', conteudo: <CamposDeConexao {...campos} /> },
    { titulo: 'Amostra', conteudo: <AmostraDeRetorno {...campos} /> },
    { titulo: 'Ajustes por condomínio', conteudo: <CamposDeAjustes {...campos} /> },
  ];
}
