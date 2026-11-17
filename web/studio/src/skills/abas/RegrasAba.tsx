import { Box, Link as MuiLink, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { Link as RouterLink } from 'react-router';
import { type Arquivo, useAgente } from '@/catalogo/contratos';
import { PESOS, RAIO, TIPOS } from '@/theme/tokens';
import { ArquivosDaSkillSecao } from './ArquivosDaSkillSecao';
import {
  documentosDe,
  documentosHerdadosDe,
  formatarTamanho,
  nomeParaExibir,
  regrasDe,
  tituloDaRegra,
} from './arquivosDaSkill';
import { RegraArquivo } from './RegraArquivo';

type Props = { nome?: string; caminho: string; titulo?: string };

export function RegrasAba({ caminho, titulo = 'Regras' }: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 3, alignItems: 'flex-start' }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <ArquivosDaSkillSecao
          caminho={caminho}
          titulo={titulo}
          ajuda="Orientações que a skill sempre segue ao responder."
          rotuloNovo="Nova regra"
          mensagemDeErro="Não foi possível carregar as regras."
          fraseVazia="Esta skill ainda não tem regras."
          tituloDaExclusao="Excluir regra"
          textoDaExclusao={(c) =>
            `Excluir a regra "${c === '' ? '' : tituloDaRegra(c)}"? A skill deixa de segui-la.`
          }
          tipo="regra"
          selecionar={(agente) => ({ proprios: regrasDe(agente), herdados: [] })}
          renderArquivo={(regra, ctx) => (
            <RegraArquivo
              key={regra.caminho}
              agente={ctx.caminhoDoAgente}
              caminho={regra.caminho}
              titulo={tituloDaRegra(regra.caminho)}
              editavel={ctx.editavel}
              onExcluir={ctx.onExcluir}
              descartar={ctx.descartar}
            />
          )}
        />
      </Box>
      <PainelDaBaseDeConhecimento caminho={caminho} />
    </Box>
  );
}

/** Rótulo overline das seções da coluna lateral: maiúsculas, tamanho do overline, cor apagada. */
const RotuloDoPainel = ({ children }: { children: ReactNode }) => (
  <Typography component="h2" variant="overline" sx={{ color: 'text.tertiary' }}>
    {children}
  </Typography>
);

/** Preview da Base de conhecimento ao lado de Regras: a aba de verdade é quem edita. */
function PainelDaBaseDeConhecimento({ caminho }: { caminho: string }): ReactNode {
  const agente = useAgente(caminho);
  if (!agente.data) return null;
  const proprios = documentosDe(agente.data.agente);
  const herdados = documentosHerdadosDe(agente.data.agente);
  if (proprios.length + herdados.length === 0) return null;

  return (
    <Box
      component="aside"
      aria-label="Base de conhecimento"
      sx={{ width: 331, flexShrink: 0, display: 'flex', flexDirection: 'column', gap: 2 }}
    >
      <Box>
        <RotuloDoPainel>Aba ao lado</RotuloDoPainel>
        <Typography sx={{ fontSize: TIPOS.h5, fontWeight: PESOS.negrito, color: 'secondary.main' }}>
          Base de conhecimento
        </Typography>
        <Typography sx={{ fontSize: TIPOS.body2, color: 'text.tertiary', mt: 1 }}>
          Documentos que a skill consulta para responder.
        </Typography>
      </Box>
      <ListaDeDocumentos rotulo="Desta skill" documentos={proprios} />
      <ListaDeDocumentos rotulo="Comuns a todas" documentos={herdados} selo="Vem do Concierge" />
      <Typography sx={{ fontSize: TIPOS.caption, color: 'text.muted' }}>
        Para mudar,{' '}
        <MuiLink component={RouterLink} to="?aba=conhecimento" sx={{ fontWeight: PESOS.semi }}>
          abra o Concierge
        </MuiLink>
        .
      </Typography>
    </Box>
  );
}

function ListaDeDocumentos({
  rotulo,
  documentos,
  selo,
}: {
  rotulo: string;
  documentos: readonly Arquivo[];
  selo?: string;
}): ReactNode {
  if (documentos.length === 0) return null;
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.25 }}>
      <RotuloDoPainel>{rotulo}</RotuloDoPainel>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {documentos.map((documento) => (
          <Box
            key={documento.caminho}
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 1.25,
              p: 1.25,
              border: 1,
              borderColor: 'border.subtle',
              borderRadius: RAIO.campo,
            }}
          >
            <Typography sx={{ flex: 1, fontSize: TIPOS.body1, fontWeight: PESOS.medio }}>
              {nomeParaExibir(documento)}
            </Typography>
            {selo && (
              <Box
                component="span"
                sx={{
                  fontSize: TIPOS.overline,
                  fontWeight: PESOS.medio,
                  py: 0.375,
                  px: 1,
                  borderRadius: RAIO.pilula,
                  bgcolor: 'background.neutralFill',
                  color: 'text.secondary',
                  whiteSpace: 'nowrap',
                }}
              >
                {selo}
              </Box>
            )}
            <Typography sx={{ fontSize: TIPOS.caption, color: 'text.muted', whiteSpace: 'nowrap' }}>
              {formatarTamanho(documento.bytes)}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}
