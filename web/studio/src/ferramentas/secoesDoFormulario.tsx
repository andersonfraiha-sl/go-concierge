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

/** Conteúdo de cada seção do drawer; a ordem aqui é a ordem na tela. */
export function secoesDoFormulario(
  control: Control<Valores>,
  travado: boolean,
  tipoInicial: string,
): { titulo: Secao; conteudo: ReactNode }[] {
  return [
    {
      titulo: 'Básico',
      conteudo: (
        <CamposDeIdentificacao control={control} travado={travado} tipoInicial={tipoInicial} />
      ),
    },
    { titulo: 'Parâmetros', conteudo: <CamposDeParametros control={control} travado={travado} /> },
    { titulo: 'Saída', conteudo: <CamposDeSaida control={control} travado={travado} /> },
    { titulo: 'Conexão', conteudo: <CamposDeConexao control={control} travado={travado} /> },
    {
      titulo: 'Amostra',
      conteudo: (
        <CampoTexto
          control={control}
          name="textoAmostra"
          label="Amostra de retorno"
          disabled={travado}
          multiline
          minRows={4}
          slotProps={{ input: { sx: { fontFamily: 'monospace' } } }}
        />
      ),
    },
    {
      titulo: 'Ajustes por condomínio',
      conteudo: <CamposDeAjustes control={control} travado={travado} />,
    },
  ];
}
