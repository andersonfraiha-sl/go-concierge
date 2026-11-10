import { Alert, Box, Chip, Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { ProblemaFluxo } from '@/fluxo/modelo/tipos';
import { PESOS, RAIO } from '@/theme/tokens';
import { ConteudoDoTrecho } from './ConteudoDoTrecho';
import type { Camada, Diferenca, TipoCamada } from './contratos';
import type { Escolha } from './decisoes';
import { EscolhaDoTrecho } from './EscolhaDoTrecho';
import { lerConteudo, type RotulosDasEtapas } from './leituraDoTrecho';

/** De onde vem cada valor: a camada de cima (quem publicou) e a da própria pessoa. */
type Origem = { de: string; quemMudou: string; sua: string };

export const ORIGENS: Record<TipoCamada, Origem> = {
  administradora: { de: 'Superlógica', quemMudou: 'A Superlógica', sua: 'Administradora' },
  condominio: { de: 'Administradora', quemMudou: 'A administradora', sua: 'Condomínio' },
};

type Props = {
  diferenca: Diferenca;
  camada: Camada;
  versaoNova: number;
  rotulosDasEtapas: RotulosDasEtapas;
  escolha: Escolha | undefined;
  problemas: readonly ProblemaFluxo[];
  editavel: boolean;
  onEscolher: (escolha: Escolha) => void;
};

// Apagar de um lado e alterar do outro nunca resolve sozinho: a tela diz qual lado apagou.
function avisoDeRemocao(d: Diferenca, origem: Origem): string | null {
  if (d.situacao !== 'conflito') return null;
  if (d.depois === null) {
    return `${origem.quemMudou} removeu este trecho, mas você tinha alterado. Ele só sai se você escolher aceitar a remoção.`;
  }
  if (d.minha === null) {
    return `Você removeu este trecho e ele foi alterado na versão nova. Escolha entre manter a remoção e aceitar a versão nova.`;
  }
  return null;
}

export function TrechoDaComparacao(props: Props): ReactNode {
  const { diferenca, camada, versaoNova, rotulosDasEtapas, problemas } = props;
  const origem = ORIGENS[camada.tipo];
  const aviso = avisoDeRemocao(diferenca, origem);
  const coluna = (valor: string | null) =>
    valor === null ? null : lerConteudo(diferenca.tipo, valor, rotulosDasEtapas);

  return (
    <Paper
      component="section"
      variant="outlined"
      aria-label={diferenca.rotulo}
      sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 1.5 }}
    >
      <CabecalhoDoTrecho diferenca={diferenca} />
      {aviso && <Alert severity="warning">{aviso}</Alert>}
      <ProblemasDoTrecho problemas={problemas} />
      <Box
        sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' } }}
      >
        <Coluna
          rotulo="Como era"
          origem={origem.de}
          conteudo={coluna(diferenca.antes)}
          vazio="Não existia"
        />
        <Coluna
          rotulo="Como ficou"
          origem={`${origem.de}, versão ${versaoNova}`}
          conteudo={coluna(diferenca.depois)}
          vazio="Removido na versão nova"
          destaque
        />
        <Coluna
          rotulo="Sua versão"
          origem={origem.sua}
          conteudo={coluna(diferenca.minha)}
          vazio={
            diferenca.antes === null ? 'Você ainda não tem este trecho' : 'Você removeu este trecho'
          }
        />
      </Box>
      <EscolhaDoTrecho
        diferenca={diferenca}
        camada={camada}
        escolha={props.escolha}
        rotulosDasEtapas={rotulosDasEtapas}
        editavel={props.editavel}
        onEscolher={props.onEscolher}
      />
    </Paper>
  );
}

function CabecalhoDoTrecho({ diferenca }: { diferenca: Diferenca }) {
  const conflito = diferenca.situacao === 'conflito';
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
      <Typography variant="subtitle2" component="h3" sx={{ flex: 1 }}>
        {diferenca.rotulo}
      </Typography>
      {diferenca.obrigatorio && <Chip size="small" color="primary" label="Obrigatório" />}
      <Chip
        size="small"
        variant="outlined"
        color={conflito ? 'warning' : 'default'}
        label={conflito ? 'Os dois mudaram' : 'Só a versão nova mudou'}
      />
    </Box>
  );
}

function ProblemasDoTrecho({ problemas }: { problemas: readonly ProblemaFluxo[] }): ReactNode {
  if (problemas.length === 0) return null;
  return (
    <Alert severity="error">
      {problemas.map((problema) => (
        <div key={`${problema.codigo}-${problema.mensagem}`}>{problema.mensagem}</div>
      ))}
    </Alert>
  );
}

type PropsDaColuna = {
  rotulo: string;
  origem: string;
  conteudo: ReturnType<typeof lerConteudo> | null;
  vazio: string;
  destaque?: boolean;
};

function Coluna({ rotulo, origem, conteudo, vazio, destaque = false }: PropsDaColuna): ReactNode {
  return (
    <Box
      role="group"
      aria-label={`${rotulo} · ${origem}`}
      sx={{
        p: 1.5,
        borderRadius: RAIO.campo,
        bgcolor: destaque ? 'background.tint' : 'background.subtle',
        minWidth: 0,
      }}
    >
      <Typography variant="caption" color="textSecondary" sx={{ display: 'block' }}>
        <Box component="span" sx={{ fontWeight: PESOS.semi }}>
          {rotulo}
        </Box>{' '}
        · {origem}
      </Typography>
      {conteudo === null ? (
        <Typography color="textSecondary" sx={{ fontStyle: 'italic' }}>
          {vazio}
        </Typography>
      ) : (
        <ConteudoDoTrecho conteudo={conteudo} />
      )}
    </Box>
  );
}
