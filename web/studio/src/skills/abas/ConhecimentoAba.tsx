import { Chip, Paper, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import type { Arquivo } from '@/catalogo/contratos';
import { ArquivosDaSkillSecao, type ContextoDoArquivo } from './ArquivosDaSkillSecao';
import {
  documentosDe,
  documentosHerdadosDe,
  ehTexto,
  formatarTamanho,
  nomeParaExibir,
  tituloDoDocumento,
} from './arquivosDaSkill';
import { DocumentoEditavel } from './DocumentoEditavel';

type Props = { nome?: string; caminho: string; titulo?: string };

export function ConhecimentoAba({ caminho, titulo = 'Base de conhecimento' }: Props): ReactNode {
  return (
    <ArquivosDaSkillSecao
      caminho={caminho}
      titulo={titulo}
      ajuda="Documentos que a skill consulta para responder."
      rotuloNovo="Novo documento"
      mensagemDeErro="Não foi possível carregar os documentos."
      fraseVazia="Esta skill ainda não tem documentos."
      tituloDaExclusao="Excluir documento"
      textoDaExclusao={(c) =>
        `Excluir "${c === '' ? '' : tituloDoDocumento(c)}"? A skill deixa de consultá-lo.`
      }
      tipo="documento"
      selecionar={(agente) => ({
        proprios: documentosDe(agente),
        herdados: documentosHerdadosDe(agente),
      })}
      renderArquivo={renderDocumento}
    />
  );
}

function renderDocumento(documento: Arquivo, ctx: ContextoDoArquivo): ReactNode {
  if (ctx.herdado) {
    return <LinhaDeArquivo key={documento.caminho} arquivo={documento} selo="Vem do Concierge" />;
  }
  if (!ehTexto(documento)) return <LinhaDeArquivo key={documento.caminho} arquivo={documento} />;
  return (
    <DocumentoEditavel
      key={documento.caminho}
      agente={ctx.caminhoDoAgente}
      caminho={documento.caminho}
      titulo={tituloDoDocumento(documento.caminho)}
      editavel={ctx.editavel}
      onExcluir={ctx.onExcluir}
      descartar={ctx.descartar}
    />
  );
}

function LinhaDeArquivo({ arquivo, selo }: { arquivo: Arquivo; selo?: string }): ReactNode {
  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1 }}>
      <Typography variant="subtitle2" sx={{ flex: 1 }}>
        {nomeParaExibir(arquivo)}
      </Typography>
      {selo && <Chip size="small" variant="outlined" label={selo} />}
      <Typography variant="caption" color="text.secondary">
        {formatarTamanho(arquivo.bytes)}
      </Typography>
    </Paper>
  );
}
