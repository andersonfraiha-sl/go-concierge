import { zodResolver } from '@hookform/resolvers/zod';
import AddIcon from '@mui/icons-material/Add';
import { Box, Button } from '@mui/material';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { paraIdentificador } from '@/skills/contratos';

const nomeDaInformacao = (texto: string): string => paraIdentificador(texto).replaceAll('-', '_');

const esquemaDaNova = (existentes: string[]) =>
  z.object({
    texto: z
      .string()
      .refine(
        (texto) => !existentes.includes(nomeDaInformacao(texto)),
        'Já existe uma informação com esse nome.',
      ),
  });

type Props = { existentes: string[]; onAdicionar: (nome: string) => void };

function useFormularioDaNova({ existentes, onAdicionar }: Props) {
  // onChange: a repetição aparece enquanto se digita, não só ao tentar adicionar.
  const { control, handleSubmit, reset } = useForm({
    mode: 'onChange',
    resolver: zodResolver(esquemaDaNova(existentes)),
    defaultValues: { texto: '' },
  });
  const nome = nomeDaInformacao(useWatch({ control, name: 'texto' }));
  const adicionar = handleSubmit(({ texto }) => {
    if (nomeDaInformacao(texto) === '') return;
    onAdicionar(nomeDaInformacao(texto));
    reset();
  });
  return { control, adicionar, indisponivel: nome === '' || existentes.includes(nome) };
}

export function NovaInformacao(props: Props): ReactNode {
  const { control, adicionar, indisponivel } = useFormularioDaNova(props);
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
      <CampoTexto
        control={control}
        name="texto"
        size="small"
        fullWidth
        label="Nome da nova informação"
        onKeyDown={(evento) => evento.key === 'Enter' && void adicionar()}
      />
      <Button
        startIcon={<AddIcon />}
        disabled={indisponivel}
        onClick={() => void adicionar()}
        sx={{ flexShrink: 0 }}
      >
        Adicionar informação
      </Button>
    </Box>
  );
}
