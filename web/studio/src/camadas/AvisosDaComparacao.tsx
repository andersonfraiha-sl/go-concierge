import { Alert, Button } from '@mui/material';
import type { ReactNode } from 'react';
import type { ProblemaFluxo } from '@/fluxo/modelo/tipos';
import type { EstadoDeAtualizacao } from './contratos';
import type { ErroDaDecisao } from './erroDaDecisao';

type Props = {
  erro: ErroDaDecisao | null;
  /** Problemas que não apontam para um trecho. */
  gerais: readonly ProblemaFluxo[];
  semPermissao: boolean;
  estado: EstadoDeAtualizacao;
  onRecarregar: () => void;
};

const AVISO_DO_ESTADO: Partial<Record<EstadoDeAtualizacao, string>> = {
  aceita: 'Esta atualização já foi aceita.',
  substituida: 'Esta atualização foi trocada por uma versão mais nova.',
};

const SEM_PERMISSAO = 'Você pode ver esta atualização, mas decidir exige permissão.';

export function AvisosDaComparacao({
  erro,
  gerais,
  semPermissao,
  estado,
  onRecarregar,
}: Props): ReactNode {
  const doEstado = AVISO_DO_ESTADO[estado];
  return (
    <>
      {semPermissao && <Alert severity="info">{SEM_PERMISSAO}</Alert>}
      {doEstado && <Alert severity="info">{doEstado}</Alert>}
      {erro && <AvisoDoErro erro={erro} onRecarregar={onRecarregar} />}
      {gerais.length > 0 && (
        <Alert severity="error">
          {gerais.map((problema) => (
            <div key={`${problema.codigo}-${problema.mensagem}`}>{problema.mensagem}</div>
          ))}
        </Alert>
      )}
    </>
  );
}

function AvisoDoErro({ erro, onRecarregar }: { erro: ErroDaDecisao; onRecarregar: () => void }) {
  switch (erro.tipo) {
    case 'desatualizada':
      return (
        <Alert
          severity="warning"
          action={
            <Button color="inherit" size="small" onClick={onRecarregar}>
              Recarregar
            </Button>
          }
        >
          Esta versão mudou. Recarregue para decidir de novo.
        </Alert>
      );
    case 'substituida':
      return (
        <Alert severity="warning">
          Chegou uma versão mais nova desta skill. Volte e revise a atualização nova.
        </Alert>
      );
    case 'problemas':
      return (
        <Alert severity="error">
          Do jeito que ficou, o passo a passo não funciona. Ajuste a mistura ou mantenha o trecho
          marcado.
        </Alert>
      );
    case 'outro':
      return <Alert severity="error">{erro.mensagem}</Alert>;
    default:
      return assertNever(erro);
  }
}

function assertNever(valor: never): never {
  throw new Error(`erro não tratado: ${JSON.stringify(valor)}`);
}
