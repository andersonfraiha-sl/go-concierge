import { Alert, Box, Button, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { usePodeEditar } from '@/acesso/eu';
import {
  CAMINHO_PERSONA,
  lerModeloDeArquivo,
  salvarArquivo,
  useAgente,
} from '@/catalogo/contratos';
import { AvisoDeConflito } from '@/copiloto/AvisoDeConflito';
import { useRealceDoCopiloto } from '@/copiloto/CopilotoProvider';
import { useEdicaoSobOServidor } from '@/copiloto/useEdicaoSobOServidor';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { type EstadoSalvamento, useAutosave } from '@/shared/autosave/useAutosave';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CONCIERGE, ESPACOS, PESOS, RAIO } from '@/theme/tokens';
import { CampoDeTextoSimples } from './CampoDeTextoSimples';
import { ConciergeCard } from './ConciergeCard';

export const TONS = {
  acolhedor: 'Fale como um vizinho prestativo e gentil: frases curtas, calorosas, sem gírias.',
  direto: 'Vá direto ao ponto: frases curtas e objetivas, sem rodeios.',
  formal: 'Fale de forma formal e respeitosa, com frases completas e sem gírias.',
  descontraido: 'Fale de um jeito descontraído e leve, com frases curtas e simpáticas.',
} as const satisfies Record<string, string>;

export const ROTULOS_DE_TOM: Record<keyof typeof TONS, string> = {
  acolhedor: 'Acolhedor',
  direto: 'Direto',
  formal: 'Formal',
  descontraido: 'Descontraído',
};

const ORDEM_DOS_TONS = ['acolhedor', 'direto', 'formal', 'descontraido'] as const satisfies Array<
  keyof typeof TONS
>;

/** Troca a frase de tom anterior (se houver) pela nova; sem frase anterior, acrescenta. */
function aplicarTom(personaAtual: string, frase: string): string {
  const anterior = Object.values(TONS).find((f) => personaAtual.includes(f));
  if (anterior !== undefined) return personaAtual.replace(anterior, frase);
  return personaAtual.trim() === '' ? frase : `${personaAtual} ${frase}`;
}

/** Tom que a persona atual reflete, se algum (a frase de um dos tons está literalmente no texto). */
function tomAtualDe(persona: string): keyof typeof TONS | null {
  return ORDEM_DOS_TONS.find((tom) => persona.includes(TONS[tom])) ?? null;
}

export function PersonaDoAgente({ assistente }: { assistente: string }): ReactNode {
  const agente = useAgente(assistente);
  if (agente.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (agente.isError) return <Falha aoTentar={() => void agente.refetch()} />;
  return <PersonaEditor assistente={assistente} personaGravada={agente.data.agente.persona} />;
}

type PropsDoEditor = { assistente: string; personaGravada: string };

/** `personaGravada` muda quando o servidor muda (ex.: proposta do Copiloto aplicada). */
function PersonaEditor({ assistente, personaGravada }: PropsDoEditor): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  const { control, persona, sincronia, estado, tentarDeNovo, usarModelo, escolherTom } =
    usePersonaEditavel({ assistente, personaGravada });
  const realce = useRealceDoCopiloto('persona');
  return (
    <ConciergeCard
      titulo="Persona e tom de voz"
      ajuda="A voz do concierge em todas as respostas ao morador."
      acao={
        <AcoesDaPersona
          editavel={editavel}
          estado={estado}
          onUsarModelo={() => void usarModelo()}
          onTentarDeNovo={() => void tentarDeNovo()}
        />
      }
    >
      <AvisoDeConflito
        conflito={sincronia.conflito}
        onManterOMeu={sincronia.manterOMeu}
        onUsarONovo={sincronia.usarONovo}
      />
      {persona.trim() === '' && (
        <Alert severity="warning">
          Sem persona, o concierge responde sem um jeito próprio de falar.
        </Alert>
      )}
      {editavel && <BotoesDeTom tomAtual={tomAtualDe(persona)} onEscolher={escolherTom} />}
      <CampoDeTextoSimples
        control={control}
        name="persona"
        rotulo="Texto da persona"
        editavel={editavel}
        minLinhas={12}
        sx={realce}
      />
    </ConciergeCard>
  );
}

function usePersonaEditavel({ assistente, personaGravada }: PropsDoEditor) {
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();
  const { control, setValue, getValues, reset } = useForm({
    defaultValues: { persona: personaGravada },
  });
  const persona = useWatch({ control, name: 'persona' });
  const sincronia = useEdicaoSobOServidor({
    servidor: personaGravada,
    alvos: ['persona'],
    versao: null,
    local: persona,
    resetar: (valor) => reset({ persona: valor }),
    salvar: async (conteudo) => {
      await salvarArquivo({ agente: assistente, caminho: CAMINHO_PERSONA, conteudo });
      await cliente.invalidateQueries({ queryKey: ['agente', assistente] });
      return null;
    },
  });
  const { estado, tentarDeNovo } = useAutosave({
    valor: persona,
    salvar: sincronia.salvar,
    bloqueio: sincronia.bloqueio,
  });

  const usarModelo = async (): Promise<void> => {
    try {
      setValue('persona', await lerModeloDeArquivo(CAMINHO_PERSONA));
    } catch (erro) {
      mostrar(erro instanceof Error ? erro.message : String(erro), 'error');
    }
  };
  const escolherTom = (tom: keyof typeof TONS): void => {
    setValue('persona', aplicarTom(getValues('persona'), TONS[tom]));
  };
  return { control, persona, sincronia, estado, tentarDeNovo, usarModelo, escolherTom };
}

type PropsDasAcoes = {
  editavel: boolean;
  estado: EstadoSalvamento;
  onUsarModelo: () => void;
  onTentarDeNovo: () => void;
};

function AcoesDaPersona({ editavel, estado, onUsarModelo, onTentarDeNovo }: PropsDasAcoes) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      {editavel && (
        <Button
          size="small"
          onClick={onUsarModelo}
          sx={{
            fontWeight: PESOS.semi,
            px: `${ESPACOS[1]}px`,
            py: 0,
            borderRadius: RAIO.chip,
          }}
        >
          Usar o modelo
        </Button>
      )}
      <SaveIndicator estado={estado} onTentarDeNovo={onTentarDeNovo} />
    </Box>
  );
}

type PropsDosBotoesDeTom = {
  tomAtual: keyof typeof TONS | null;
  onEscolher: (tom: keyof typeof TONS) => void;
};

function BotoesDeTom({ tomAtual, onEscolher }: PropsDosBotoesDeTom): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
      {ORDEM_DOS_TONS.map((tom) => (
        <BotaoDeTom
          key={tom}
          selecionado={tom === tomAtual}
          onClick={() => onEscolher(tom)}
          rotulo={ROTULOS_DE_TOM[tom]}
        />
      ))}
    </Box>
  );
}

function BotaoDeTom({
  selecionado,
  onClick,
  rotulo,
}: {
  selecionado: boolean;
  onClick: () => void;
  rotulo: string;
}): ReactNode {
  return (
    <Button
      size="small"
      aria-pressed={selecionado}
      onClick={onClick}
      sx={{
        borderRadius: RAIO.pilula,
        px: `${ESPACOS[2]}px`,
        py: `${CONCIERGE.paddingYDaPilula}px`,
        border: 1,
        borderColor: selecionado ? 'primary.main' : 'border.strong',
        bgcolor: selecionado ? 'background.tint' : 'background.paper',
        color: selecionado ? 'primary.main' : 'text.secondary',
        fontWeight: selecionado ? PESOS.semi : PESOS.regular,
      }}
    >
      {rotulo}
    </Button>
  );
}

export function Falha({ aoTentar }: { aoTentar: () => void }): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Typography role="alert">Não foi possível carregar a persona.</Typography>
      <Button size="small" onClick={aoTentar}>
        Tentar de novo
      </Button>
    </Box>
  );
}
