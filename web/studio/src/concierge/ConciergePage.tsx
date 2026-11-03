import { Alert, Box, Button, Tab, Tabs, Typography } from '@mui/material';
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
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';
import { useSkills } from '@/skills/contratos';

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

export function ConciergePage(): ReactNode {
  const skills = useSkills();
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Typography variant="h5" component="h1">
        Concierge
      </Typography>
      <Tabs value="persona" aria-label="Seções do Concierge">
        <Tab value="persona" label="Persona" />
      </Tabs>
      {skills.isPending && <Typography color="text.secondary">Carregando…</Typography>}
      {skills.isError && <Falha aoTentar={() => void skills.refetch()} />}
      {skills.data && <PersonaDoAgente assistente={skills.data.assistente} />}
    </Box>
  );
}

function PersonaDoAgente({ assistente }: { assistente: string }): ReactNode {
  const agente = useAgente(assistente);
  if (agente.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (agente.isError) return <Falha aoTentar={() => void agente.refetch()} />;
  return <PersonaEditor assistente={assistente} personaInicial={agente.data.agente.persona} />;
}

function PersonaEditor({
  assistente,
  personaInicial,
}: {
  assistente: string;
  personaInicial: string;
}): ReactNode {
  const editavel = usePodeEditar('skills.editar_textos');
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();
  const { control, setValue, getValues } = useForm({ defaultValues: { persona: personaInicial } });
  const persona = useWatch({ control, name: 'persona' });
  const { estado, tentarDeNovo } = useAutosave({
    valor: persona,
    salvar: async (conteudo) => {
      await salvarArquivo({ agente: assistente, caminho: CAMINHO_PERSONA, conteudo });
      await cliente.invalidateQueries({ queryKey: ['agente', assistente] });
    },
  });

  const usarModelo = async (): Promise<void> => {
    try {
      const conteudo = await lerModeloDeArquivo(CAMINHO_PERSONA);
      setValue('persona', conteudo);
    } catch (erro) {
      mostrar(erro instanceof Error ? erro.message : String(erro), 'error');
    }
  };

  const escolherTom = (tom: keyof typeof TONS): void => {
    setValue('persona', aplicarTom(getValues('persona'), TONS[tom]));
  };

  return (
    <SectionCard
      titulo="Persona"
      ajuda="A voz do concierge em todas as respostas ao morador."
      acao={
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          {editavel && (
            <Button size="small" onClick={() => void usarModelo()}>
              Usar o modelo
            </Button>
          )}
          <SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />
        </Box>
      }
    >
      {persona.trim() === '' && (
        <Alert severity="warning">
          Sem persona, o concierge responde sem um jeito próprio de falar.
        </Alert>
      )}
      {editavel && (
        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
          {ORDEM_DOS_TONS.map((tom) => (
            <Button key={tom} size="small" variant="outlined" onClick={() => escolherTom(tom)}>
              {ROTULOS_DE_TOM[tom]}
            </Button>
          ))}
        </Box>
      )}
      <CampoTexto
        control={control}
        name="persona"
        multiline
        minRows={12}
        fullWidth
        slotProps={{ htmlInput: { readOnly: !editavel, 'aria-label': 'Texto da persona' } }}
      />
    </SectionCard>
  );
}

function Falha({ aoTentar }: { aoTentar: () => void }): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Typography role="alert">Não foi possível carregar a persona.</Typography>
      <Button size="small" onClick={aoTentar}>
        Tentar de novo
      </Button>
    </Box>
  );
}
