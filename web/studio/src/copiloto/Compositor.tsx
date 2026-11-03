import { zodResolver } from '@hookform/resolvers/zod';
import ArrowForwardRounded from '@mui/icons-material/ArrowForwardRounded';
import { Box, IconButton, InputBase, Typography } from '@mui/material';
import type { KeyboardEvent } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { RAIOS, TIPOS, tokens } from '@/theme/tokens';
import { LIMITE_DA_MENSAGEM } from './contratos';

type Props = { desabilitado: boolean; aoEnviar: (texto: string) => Promise<void> };

const MOSTRA_CONTADOR_ACIMA_DE = 3500;

const FormularioSchema = z.object({
  texto: z.string().trim().min(1).max(LIMITE_DA_MENSAGEM),
});
const rodape = { fontSize: TIPOS.overline, color: tokens.color.text.muted } as const;

export function Compositor({ desabilitado, aoEnviar }: Props) {
  const { handleSubmit, reset, control } = useForm({
    resolver: zodResolver(FormularioSchema),
    defaultValues: { texto: '' },
  });
  const tamanho = useWatch({ control, name: 'texto' }).length;

  const enviar = handleSubmit(({ texto }) => {
    reset();
    return aoEnviar(texto);
  });

  // Enter envia, Shift+Enter quebra linha; durante composição de acento (IME) o Enter é do teclado.
  const aoTeclar = (evento: KeyboardEvent) => {
    if (evento.key !== 'Enter' || evento.shiftKey || evento.nativeEvent.isComposing) return;
    evento.preventDefault();
    void enviar();
  };

  return (
    <Box sx={{ px: 3, pt: 2, pb: 3, borderTop: `1px solid ${tokens.color.border.subtle}` }}>
      <Box
        component="form"
        onSubmit={enviar}
        sx={{
          display: 'flex',
          alignItems: 'flex-end',
          gap: 1,
          border: 1,
          borderColor: tokens.color.border.strong,
          borderRadius: `${RAIOS.painel}px`,
          p: 1,
          pl: 2,
          bgcolor: desabilitado ? tokens.color.background.subtle : 'background.paper',
          '&:focus-within': { borderColor: 'primary.main' },
        }}
      >
        {/* Controller, e não register: o InputBase chama onBlur sem evento ao ser desabilitado com foco. */}
        <Controller
          control={control}
          name="texto"
          render={({ field: { ref, ...campo } }) => (
            <InputBase
              {...campo}
              inputRef={ref}
              multiline
              minRows={2}
              maxRows={8}
              fullWidth
              disabled={desabilitado}
              onKeyDown={aoTeclar}
              placeholder="Peça uma mudança, uma explicação ou uma skill nova…"
              slotProps={{
                input: { 'aria-label': 'Mensagem para o copiloto', maxLength: LIMITE_DA_MENSAGEM },
              }}
              sx={{ fontSize: 13, lineHeight: 1.5 }}
            />
          )}
        />
        <IconButton
          type="submit"
          aria-label="Enviar"
          disabled={desabilitado}
          sx={{
            width: 34,
            height: 34,
            borderRadius: `${RAIOS.chip}px`,
            bgcolor: 'primary.main',
            color: 'primary.contrastText',
            '&:hover': { bgcolor: 'primary.dark' },
          }}
        >
          <ArrowForwardRounded fontSize="small" />
        </IconButton>
      </Box>
      <Box sx={{ display: 'flex', mt: 1, gap: 1 }}>
        <Typography sx={{ ...rodape, flex: 1 }}>
          Enter envia · Shift+Enter quebra linha · nada muda sem você aplicar
        </Typography>
        {tamanho > MOSTRA_CONTADOR_ACIMA_DE && (
          <Typography sx={rodape}>
            {tamanho}/{LIMITE_DA_MENSAGEM}
          </Typography>
        )}
      </Box>
    </Box>
  );
}
