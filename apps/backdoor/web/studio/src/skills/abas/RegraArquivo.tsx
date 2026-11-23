import { Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useArquivo } from '@/catalogo/contratos';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { DocumentoEditavel } from './DocumentoEditavel';
import { SecoesDeRegra } from './SecoesDeRegra';
import { dividirEmSecoes } from './secoesDoMarkdown';

type Props = {
  agente: string;
  caminho: string;
  titulo: string;
  editavel: boolean;
  onExcluir?: () => void;
  descartar?: boolean;
};

/**
 * Um arquivo de regra com título `## ` dentro vira um cartão por título (SecoesDeRegra), com o
 * nome exato do título; sem nenhum `## `, continua um cartão só pelo nome do arquivo, como
 * sempre (DocumentoEditavel).
 */
export function RegraArquivo(props: Props): ReactNode {
  const { agente, caminho, titulo, onExcluir, descartar } = props;
  const arquivo = useArquivo(agente, caminho);

  if (arquivo.isPending) return <Typography variant="caption">Carregando {titulo}…</Typography>;
  if (arquivo.isError) {
    return (
      <ErroComRetry
        mensagem={`Não foi possível abrir ${titulo}.`}
        onTentarDeNovo={() => void arquivo.refetch()}
      />
    );
  }

  const secoes = dividirEmSecoes(arquivo.data.conteudo);
  if (secoes.length === 0) return <DocumentoEditavel {...props} />;
  return (
    <SecoesDeRegra
      agente={agente}
      caminho={caminho}
      secoes={secoes}
      editavel={props.editavel}
      onExcluirArquivo={onExcluir}
      descartar={descartar}
    />
  );
}
