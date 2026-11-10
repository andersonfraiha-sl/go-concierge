import { Box, Link as MuiLink, Typography } from '@mui/material';
import type { ComponentProps, ReactNode } from 'react';
import { type Control, Controller, useWatch } from 'react-hook-form';
import { Link as RouterLink } from 'react-router';
import { type Estado, habilitacaoDe, useEstado } from '@/condominios/contratos';
import { type EstiloDoRealce, useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import type { Informacao } from '@/fluxo/modelo/tipos';
import { TIPOS_DE_INFORMACAO, tipoMostrado } from '@/fluxo/painel/InformacoesPainel';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';
import { PESOS, RAIO, TIPOS } from '@/theme/tokens';
import { passoDaInformacao } from './abas/fluxoDaSkill';
import { LIMITE_DO_RESUMO, ResumoSchema, type Textos, useFluxo, useSkills } from './contratos';
import { ListaDeChips } from './ListaDeChips';

type Props = {
  nome: string;
  control: Control<Textos>;
  editavel: boolean;
};

/** Realce no campo de texto: o fundo do input, não o cartão branco em volta. */
const noCampo = (realce: EstiloDoRealce) => ({ '& .MuiInputBase-root': realce });
/** Realce na lista: o cartão inteiro, já que a mudança pode ser qualquer chip. */
const naLista = (realce: EstiloDoRealce) => ({ '& > .MuiPaper-root': realce });

type PropsDaAba = Props & {
  /** O painel "Esta skill" descreve a base (onde está ligada, o que pede): a camada não o mostra. */
  comPainel?: boolean;
};

export function QuandoUsarAba({ comPainel = true, ...props }: PropsDaAba): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 3, alignItems: 'flex-start' }}>
      <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <SecaoDoResumo {...props} />
        <SecaoQuandoUsar {...props} />
        <ListaDoCopiloto {...props} campo="palavrasChave" />
        <ListaDoCopiloto {...props} campo="frasesDeExemplo" />
      </Box>
      {comPainel && <PainelDaSkill {...props} />}
    </Box>
  );
}

function SecaoDoResumo({ nome, control, editavel }: Props): ReactNode {
  const resumo = useWatch({ control, name: 'resumo' });
  const realce = useRealceDoCopiloto(`skill:${nome}:resumo`);
  // Um resumo gravado fora da regra (texto antigo, outra ferramenta) avisa aqui em vez de passar calado.
  const foraDaRegra = ResumoSchema.safeParse(resumo).error?.issues[0]?.message;
  return (
    <SectionCard
      titulo="Resumo"
      ajuda="Uma linha para as pessoas entenderem o que a skill faz. Aparece para as administradoras."
    >
      <CampoTexto
        control={control}
        name="resumo"
        fullWidth
        disabled={!editavel}
        helperText={foraDaRegra ?? `${resumo.length}/${LIMITE_DO_RESUMO}`}
        sx={[
          noCampo(realce),
          foraDaRegra === undefined && {
            position: 'relative',
            '& .MuiFormHelperText-root': {
              position: 'absolute',
              right: 0,
              bottom: 0,
              m: 0,
              px: 1.5,
              py: 1,
              pointerEvents: 'none',
            },
          },
        ]}
        slotProps={{ htmlInput: { 'aria-label': 'Resumo', maxLength: LIMITE_DO_RESUMO } }}
      />
    </SectionCard>
  );
}

function SecaoQuandoUsar({ nome, control, editavel }: Props): ReactNode {
  const realce = useRealceDoCopiloto(`skill:${nome}:quandoUsar`);
  return (
    <SectionCard
      titulo="Como reconhecer o pedido"
      ajuda="Para o concierge decidir quando usar esta skill: em que situação o morador pede isto."
    >
      <CampoTexto
        control={control}
        name="quandoUsar"
        multiline
        minRows={2}
        fullWidth
        disabled={!editavel}
        sx={noCampo(realce)}
        slotProps={{ htmlInput: { 'aria-label': 'Como reconhecer o pedido' } }}
      />
    </SectionCard>
  );
}

type CampoDeLista = 'palavrasChave' | 'frasesDeExemplo';

const TEXTOS_DAS_LISTAS: Record<
  CampoDeLista,
  Pick<
    ComponentProps<typeof ListaDeChips>,
    'titulo' | 'ajuda' | 'rotuloNovoItem' | 'placeholderNovoItem' | 'mensagemVazia' | 'comAspas'
  >
> = {
  palavrasChave: {
    titulo: 'Palavras-chave',
    ajuda: 'Palavras que ajudam a reconhecer o pedido.',
    rotuloNovoItem: 'Nova palavra-chave',
    placeholderNovoItem: 'Adicionar palavra e apertar Enter',
    mensagemVazia: 'Nenhuma palavra-chave.',
  },
  frasesDeExemplo: {
    titulo: 'Frases de exemplo',
    ajuda: 'Frases que um morador diria para pedir isto.',
    rotuloNovoItem: 'Nova frase de exemplo',
    placeholderNovoItem: 'Adicionar frase e apertar Enter',
    mensagemVazia: 'Nenhuma frase de exemplo.',
    comAspas: true,
  },
};

function ListaDoCopiloto({ nome, control, editavel, campo }: Props & { campo: CampoDeLista }) {
  const realce = useRealceDoCopiloto(`skill:${nome}:${campo}`);
  return (
    <Controller
      control={control}
      name={campo}
      render={({ field }) => (
        <Box sx={naLista(realce)}>
          <ListaDeChips
            {...TEXTOS_DAS_LISTAS[campo]}
            itens={field.value}
            editavel={editavel}
            onMudar={field.onChange}
          />
        </Box>
      )}
    />
  );
}

/** Rótulo overline das seções da coluna "Esta skill": maiúsculas, tamanho do overline, cor apagada. */
const RotuloDoPainel = ({ children }: { children: ReactNode }) => (
  <Typography component="h2" variant="overline" sx={{ color: 'text.tertiary' }}>
    {children}
  </Typography>
);

/** Subtítulo de uma informação pedida: as opções de Escolha, senão o tipo de campo. */
function subtituloDaInformacao(info: Informacao): string {
  const tipo = tipoMostrado(info.tipo);
  if (tipo === 'select' && info.opcoes.length > 0) {
    return info.opcoes.map((opcao) => opcao.rotulo).join(', ');
  }
  return TIPOS_DE_INFORMACAO[tipo] ?? tipo;
}

type PropsDasInformacoes = { nome: string };

/** "Informações que a skill pede": as perguntas do Fluxo, com o passo em que cada uma aparece. */
function InformacoesDoFluxo({ nome }: PropsDasInformacoes): ReactNode {
  const fluxo = useFluxo(nome);
  const informacoes = fluxo.data?.fluxo.informacoes ?? [];
  if (informacoes.length === 0) return null;
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.25 }}>
      <RotuloDoPainel>Informações que a skill pede</RotuloDoPainel>
      <Box sx={{ border: 1, borderColor: 'border.subtle', borderRadius: RAIO.campo }}>
        {informacoes.map((info) => (
          <LinhaDaInformacao
            key={info.nome}
            info={info}
            passo={passoDaInformacao(fluxo.data?.fluxo.blocos ?? [], info.nome)}
          />
        ))}
      </Box>
      <Typography variant="caption" sx={{ color: 'text.muted' }}>
        Vêm das perguntas do Fluxo. Para mudar,{' '}
        <MuiLink component={RouterLink} to="../fluxo" sx={{ fontWeight: PESOS.semi }}>
          abra o Fluxo
        </MuiLink>
        .
      </Typography>
    </Box>
  );
}

function LinhaDaInformacao({ info, passo }: { info: Informacao; passo: number | undefined }) {
  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.25,
        px: 1.5,
        py: 1.25,
        '&:not(:last-of-type)': { borderBottom: 1, borderColor: 'border.subtle' },
      }}
    >
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography sx={{ fontSize: TIPOS.body1, fontWeight: PESOS.semi }}>
          {info.titulo || info.nome}
        </Typography>
        <Typography sx={{ fontSize: TIPOS.caption, color: 'text.muted', mt: 0.25 }}>
          {subtituloDaInformacao(info)}
        </Typography>
      </Box>
      {passo !== undefined && (
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
          passo {passo}
        </Box>
      )}
    </Box>
  );
}

/** Quantos condomínios ligam esta skill (decisão própria ou herdada do Padrão). */
function contarCondominiosLigados(
  estado: Estado,
  caso: string,
): { ligados: number; total: number } {
  const condominios = estado.mapeamento.condominios;
  const ligados = condominios.filter(
    (c) => habilitacaoDe(estado.mapeamento, c.id, caso).habilitacao.habilitado,
  ).length;
  return { ligados, total: condominios.length };
}

/** "Onde está ligada": em quantos condomínios a skill está ativa. */
function OndeEstaLigada({ nome }: PropsDasInformacoes): ReactNode {
  const estado = useEstado();
  if (!estado.data) return null;
  const { ligados, total } = contarCondominiosLigados(estado.data, nome);
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.25 }}>
      <RotuloDoPainel>Onde está ligada</RotuloDoPainel>
      <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1 }}>
        <Typography sx={{ fontSize: TIPOS.h4, fontWeight: PESOS.negrito, color: 'secondary.main' }}>
          {ligados}
        </Typography>
        <Typography sx={{ fontSize: TIPOS.body2, color: 'text.secondary' }}>
          de {total} condomínios
        </Typography>
      </Box>
      <MuiLink
        component={RouterLink}
        to="/condominios"
        sx={{ fontSize: TIPOS.body2, fontWeight: PESOS.semi }}
      >
        Ver por condomínio
      </MuiLink>
    </Box>
  );
}

/** Coluna lateral "Esta skill": o que ela pede, quem pode usar, onde está ligada e se grava algo. */
function PainelDaSkill({ nome, control }: Props): ReactNode {
  const visibilidade = useWatch({ control, name: 'visibilidade' });
  const skill = useSkills().data?.skills.find((s) => s.nome === nome);
  const visivelParaTodos = visibilidade === 'public';

  return (
    <Box
      component="aside"
      sx={{ width: 331, flexShrink: 0, display: 'flex', flexDirection: 'column', gap: 2 }}
    >
      <RotuloDoPainel>Esta skill</RotuloDoPainel>
      <InformacoesDoFluxo nome={nome} />
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        <RotuloDoPainel>Quem pode usar</RotuloDoPainel>
        <Box
          component="span"
          sx={{
            alignSelf: 'flex-start',
            fontSize: TIPOS.caption,
            fontWeight: PESOS.medio,
            py: 0.5,
            px: 1.25,
            borderRadius: RAIO.pilula,
            bgcolor: 'background.tint',
            color: 'primary.main',
          }}
        >
          {visivelParaTodos ? 'Qualquer contato' : 'Morador identificado'}
        </Box>
      </Box>
      <OndeEstaLigada nome={nome} />
      {skill?.escrita && (
        <Box
          sx={{
            bgcolor: 'studio.registra.soft',
            color: 'studio.registra.onSoft',
            borderRadius: RAIO.menu,
            p: 1.75,
          }}
        >
          <RotuloDoPainel>Grava no sistema</RotuloDoPainel>
          <Typography sx={{ fontSize: TIPOS.body2, mt: 0.75 }}>
            Esta skill registra algo no sistema ao final.
          </Typography>
        </Box>
      )}
    </Box>
  );
}
