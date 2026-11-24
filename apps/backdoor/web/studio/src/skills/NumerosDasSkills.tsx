import { Box, CircularProgress } from '@mui/material';
import type { ReactNode } from 'react';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { CartaoDeNumero } from './CartaoDeNumero';
import type { SkillResumo } from './contratos';
import { type Problemas, quartaEstatistica } from './estatisticas';

type Props = {
  skills: readonly SkillResumo[];
  problemas: ReadonlyMap<string, Problemas>;
  podeVerProblemas: boolean;
  arvore: { isPending: boolean; isError: boolean; error: Error | null; refetch: () => unknown };
};

/** 4º cartão; "Com problema" depende da árvore, então não mostra número antes de ela carregar. */
function QuartoCartao({ skills, problemas, podeVerProblemas, arvore }: Props): ReactNode {
  if (podeVerProblemas && arvore.isError) {
    return (
      <ErroComRetry
        mensagem={arvore.error?.message ?? 'Não foi possível carregar os problemas.'}
        onTentarDeNovo={() => void arvore.refetch()}
      />
    );
  }
  if (podeVerProblemas && arvore.isPending) {
    return (
      <CartaoDeNumero
        rotulo="Com problema"
        valor={<CircularProgress size={22} aria-label="Carregando problemas" />}
      />
    );
  }
  return <CartaoDeNumero {...quartaEstatistica(skills, problemas, podeVerProblemas)} />;
}

/** Os 4 cartões de número do topo; os totais não mudam com a busca. */
export function NumerosDasSkills(props: Props): ReactNode {
  const { skills } = props;
  const escreventes = skills.filter((skill) => skill.escrita).length;
  return (
    <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 2 }}>
      <CartaoDeNumero rotulo="Skills" valor={skills.length} />
      <CartaoDeNumero rotulo="Registram no sistema" valor={escreventes} />
      <CartaoDeNumero rotulo="Consultas e dúvidas" valor={skills.length - escreventes} />
      <QuartoCartao {...props} />
    </Box>
  );
}
