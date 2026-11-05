import DescriptionOutlinedIcon from '@mui/icons-material/DescriptionOutlined';
import { Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import {
  documentosDe,
  formatarTamanho,
  nomeParaExibir,
  tituloDoDocumento,
} from '@/skills/abas/arquivosDaSkill';
import { ICONES, PESOS } from '@/theme/tokens';
import { BotaoExcluir, type ItemDaLista, ListaDeArquivosCard } from './ListaDeArquivosCard';

const TEXTOS = {
  erro: 'Não foi possível carregar os documentos.',
  vazio: 'Esta skill ainda não tem documentos.',
  excluir: (caminho: string) =>
    `Excluir "${tituloDoDocumento(caminho)}"? A skill deixa de consultá-lo.`,
};

const extensaoDe = (caminho: string): string => {
  const ponto = caminho.lastIndexOf('.');
  return ponto === -1 ? '' : caminho.slice(ponto + 1).toUpperCase();
};

/** Base de conhecimento como linhas fechadas: nome, tipo e tamanho, e excluir. */
export function BaseDeConhecimentoCard({ assistente }: { assistente: string }): ReactNode {
  return (
    <ListaDeArquivosCard
      assistente={assistente}
      tipo="documento"
      titulo="Base de conhecimento comum"
      ajuda="Documentos que todas as skills consultam."
      rotuloDoNovo="Novo documento"
      textos={TEXTOS}
      arquivosDe={documentosDe}
      item={(props) => <LinhaDeDocumento {...props} />}
    />
  );
}

function LinhaDeDocumento({ arquivo, emExclusao, onExcluir }: ItemDaLista): ReactNode {
  const nome = nomeParaExibir(arquivo);
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 2,
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        borderColor: 'border.default',
        opacity: emExclusao ? 0.5 : 1,
      }}
    >
      <DescriptionOutlinedIcon sx={{ fontSize: ICONES.grande, color: 'text.tertiary' }} />
      <Typography variant="body1" sx={{ flex: 1, fontWeight: PESOS.semi }}>
        {nome}
      </Typography>
      <Typography variant="body2" color="textSecondary">
        {extensaoDe(arquivo.caminho)} · {formatarTamanho(arquivo.bytes)}
      </Typography>
      {onExcluir && <BotaoExcluir nome={nome} emExclusao={emExclusao} onExcluir={onExcluir} />}
    </Paper>
  );
}
