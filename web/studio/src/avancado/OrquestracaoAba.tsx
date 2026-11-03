import { zodResolver } from '@hookform/resolvers/zod';
import {
  Alert,
  Box,
  Button,
  FormControlLabel,
  Radio,
  RadioGroup,
  TextField,
  Typography,
} from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { z } from 'zod';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  LIMITES_DE_ORQUESTRACAO,
  MODOS,
  MODOS_DE_ORQUESTRACAO,
  mensagemDeErro,
  type Orquestracao,
  type OrquestracaoLida,
  salvarOrquestracao,
  useOrquestracao,
  validarOrquestracao,
} from './contratos';

const RascunhoSchema = z
  .object({
    modo: z.enum(MODOS_DE_ORQUESTRACAO),
    bind_confidence: z.string(),
    max_itens: z.string(),
    max_iteracoes: z.string(),
  })
  .superRefine((rascunho, contexto) => {
    for (const { campo, mensagem } of validarOrquestracao(ajusteDe(rascunho))) {
      contexto.addIssue({ code: 'custom', path: [campo], message: mensagem });
    }
  });
type Rascunho = z.infer<typeof RascunhoSchema>;

const rascunhoDe = (ajuste: Orquestracao): Rascunho => ({
  modo: ajuste.modo,
  bind_confidence: String(ajuste.bind_confidence),
  max_itens: String(ajuste.max_itens),
  max_iteracoes: String(ajuste.max_iteracoes),
});

const ajusteDe = (rascunho: Rascunho): Orquestracao => ({
  modo: rascunho.modo,
  bind_confidence: Number(rascunho.bind_confidence.replace(',', '.')),
  max_itens: Number(rascunho.max_itens),
  max_iteracoes: Number(rascunho.max_iteracoes),
});

export function OrquestracaoAba({ editar }: { editar: boolean }): ReactNode {
  const orquestracao = useOrquestracao();
  if (orquestracao.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (orquestracao.isError) {
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography role="alert">Não foi possível carregar o ajuste.</Typography>
        <Button size="small" onClick={() => void orquestracao.refetch()}>
          Tentar de novo
        </Button>
      </Box>
    );
  }
  return <Formulario lida={orquestracao.data} editar={editar} />;
}

function Formulario({ lida, editar }: { lida: OrquestracaoLida; editar: boolean }): ReactNode {
  const { mostrar } = useSnackbar();
  const cliente = useQueryClient();
  const { control, handleSubmit, formState } = useForm({
    resolver: zodResolver(RascunhoSchema),
    defaultValues: rascunhoDe(lida.arquivo),
  });
  const problemas = LIMITES_DE_ORQUESTRACAO.flatMap(
    ({ campo }) => formState.errors[campo]?.message ?? [],
  );

  const salvar = handleSubmit(async (rascunho) => {
    try {
      const gravada = await salvarOrquestracao(ajusteDe(rascunho));
      await cliente.invalidateQueries({ queryKey: ['avancado', 'orquestracao'] });
      mostrar(avisoDeSalvo(gravada));
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  });

  return (
    <SectionCard
      titulo="Orquestração"
      ajuda="Como o concierge decide o que fazer com cada mensagem."
      acao={
        editar && (
          <Button
            variant="contained"
            disabled={formState.isSubmitting}
            onClick={() => void salvar()}
          >
            Salvar
          </Button>
        )
      }
    >
      {lida.ambiente !== '' && (
        <Alert severity="warning">
          O ambiente está forçando o modo {lida.ambiente}. O que você salvar aqui só vale quando
          essa trava sair.
        </Alert>
      )}
      <Controller
        control={control}
        name="modo"
        render={({ field }) => (
          <RadioGroup
            aria-label="Modo"
            value={field.value}
            onChange={(_, valor) => {
              const modo = MODOS_DE_ORQUESTRACAO.find((m) => m === valor);
              if (modo) field.onChange(modo);
            }}
          >
            {MODOS_DE_ORQUESTRACAO.map((modo) => (
              <FormControlLabel
                key={modo}
                value={modo}
                disabled={!editar}
                control={<Radio />}
                label={
                  <Box>
                    <Typography variant="body2" sx={{ fontWeight: 700 }}>
                      {MODOS[modo].nome}
                      {lida.efetivo.modo === modo && ' (em uso)'}
                    </Typography>
                    <Typography variant="caption" color="text.secondary">
                      {MODOS[modo].descricao}
                    </Typography>
                    <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                      Quando: {MODOS[modo].quando}
                    </Typography>
                  </Box>
                }
              />
            ))}
          </RadioGroup>
        )}
      />
      <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
        {LIMITES_DE_ORQUESTRACAO.map(({ campo, rotulo, min, max, passo, ajuda }) => (
          <Controller
            key={campo}
            control={control}
            name={campo}
            render={({ field }) => (
              <TextField
                type="number"
                size="small"
                label={rotulo}
                helperText={ajuda}
                value={field.value}
                onChange={field.onChange}
                slotProps={{ htmlInput: { min, max, step: passo, readOnly: !editar } }}
              />
            )}
          />
        ))}
      </Box>
      {problemas.map((problema) => (
        <Typography key={problema} role="alert" color="error" variant="body2">
          {problema}
        </Typography>
      ))}
    </SectionCard>
  );
}

const avisoDeSalvo = (gravada: OrquestracaoLida): string =>
  gravada.ambiente === ''
    ? `Salvo: modo ${gravada.efetivo.modo} a partir da próxima mensagem`
    : `Salvo, mas o ambiente ainda força o modo ${gravada.efetivo.modo}`;
