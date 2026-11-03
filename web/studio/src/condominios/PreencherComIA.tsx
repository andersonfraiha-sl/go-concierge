import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button, TextField, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import type { Ferramenta } from '@/catalogo/contratos';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { sugerirAjustes, useEscopo, type Valor } from './contratos';

type Props = {
  condominio: string;
  ferramentas: Ferramenta[];
  valores: Record<string, Record<string, Valor>>;
  onPreencher: (recurso: string, campos: Record<string, Valor>) => void;
};

const FraseSchema = z.object({ frase: z.string().refine((frase) => frase.trim() !== '') });

/** Botão único que abre a caixa de sugestão por IA para os ajustes gerais do condomínio. */
export function PreencherComIA({
  condominio,
  ferramentas,
  valores,
  onPreencher,
}: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const [aberto, setAberto] = useState(false);
  const [recurso, setRecurso] = useState(() => ferramentas[0]?.name ?? '');
  const [aviso, setAviso] = useState<string | null>(null);
  const { control, handleSubmit, formState } = useForm({
    resolver: zodResolver(FraseSchema),
    defaultValues: { frase: '' },
  });
  const frase = useWatch({ control, name: 'frase' });

  if (ferramentas.length === 0) return null;
  if (!aberto) {
    return (
      <Button variant="outlined" onClick={() => setAberto(true)}>
        Preencher com IA
      </Button>
    );
  }

  const sugerir = handleSubmit(async ({ frase }) => {
    setAviso(null);
    try {
      const sugestao = await sugerirAjustes(
        { condominio, recurso, texto: frase, atual: valores[recurso] ?? {} },
        escopo,
      );
      onPreencher(recurso, sugestao.payload);
      const semNada = Object.keys(sugestao.payload).length === 0;
      setAviso(
        sugestao.nota ||
          (semNada ? 'Não encontrei nada no texto.' : 'Revise os campos preenchidos.'),
      );
    } catch (erro) {
      setAviso(erro instanceof Error ? erro.message : String(erro));
    }
  });
  const pensando = formState.isSubmitting;

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      {ferramentas.length > 1 && (
        <TextField
          select
          label="Ferramenta"
          size="small"
          value={recurso}
          onChange={(e) => setRecurso(e.target.value)}
          slotProps={{ select: { native: true } }}
        >
          {ferramentas.map((ferramenta) => (
            <option key={ferramenta.name} value={ferramenta.name}>
              {ferramenta.titulo ?? ferramenta.name}
            </option>
          ))}
        </TextField>
      )}
      <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
        <CampoTexto
          control={control}
          name="frase"
          size="small"
          fullWidth
          label="Descreva em uma frase"
        />
        <Button
          variant="outlined"
          disabled={pensando || frase.trim() === ''}
          onClick={() => void sugerir()}
        >
          {pensando ? 'Pensando…' : 'Sugerir'}
        </Button>
      </Box>
      {aviso !== null && (
        <Typography variant="caption" color="text.secondary" role="status">
          {aviso}
        </Typography>
      )}
    </Box>
  );
}
