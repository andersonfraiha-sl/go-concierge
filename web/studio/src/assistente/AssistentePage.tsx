import { Box, Button, Tab, Tabs, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { usePodeEditar } from '@/acesso/eu';
import { CAMINHO_PERSONA, salvarArquivo, useAgente } from '@/catalogo/contratos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';
import { useSkills } from '@/skills/contratos';

export function AssistentePage(): ReactNode {
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
  const { control } = useForm({ defaultValues: { persona: personaInicial } });
  const { estado, tentarDeNovo } = useAutosave({
    valor: useWatch({ control, name: 'persona' }),
    salvar: async (conteudo) => {
      await salvarArquivo({ agente: assistente, caminho: CAMINHO_PERSONA, conteudo });
      await cliente.invalidateQueries({ queryKey: ['agente', assistente] });
    },
  });

  return (
    <SectionCard
      titulo="Persona"
      ajuda="A voz da assistente em todas as respostas ao morador."
      acao={<SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
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
