import { zodResolver } from '@hookform/resolvers/zod';
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Typography,
} from '@mui/material';
import type { FormEvent, ReactNode } from 'react';
import { useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { LIMITE_DO_RESUMO, paraIdentificador } from './contratos';

export type SkillNova = { nome: string; titulo: string; resumo: string; quandoUsar: string };
type TextosDaNova = { resumo: string; quandoUsar: string };

type Props = {
  /** POST da skill; rejeita com Error cuja mensagem aparece sob o campo Nome. */
  onCriar: (nova: { nome: string; titulo: string }) => Promise<{ nome: string }>;
  /** PUT dos textos da skill já criada; só este passo se repete em "Tentar de novo". */
  onSalvarTextos: (nome: string, textos: TextosDaNova) => Promise<void>;
  /** Tudo certo: a pessoa segue para o editor. */
  onConcluida: (nome: string) => void;
  /** Skill criada mas os textos não salvaram, e a pessoa fechou assim mesmo: segue para o editor sem perder o que digitou. */
  onFecharComFalha: (nome: string, textos: TextosDaNova) => void;
  /** Fechar antes de a skill existir: só fecha o diálogo. */
  onCancelar: () => void;
};

const NovaSkillSchema = z.object({
  titulo: z
    .string()
    .refine((titulo) => paraIdentificador(titulo) !== '', 'Use letras ou números no nome.'),
  resumo: z.string().max(LIMITE_DO_RESUMO),
  quandoUsar: z.string(),
});

const MENSAGEM_TEXTOS_FALHARAM = 'A skill foi criada, mas os textos não salvaram. Tente de novo.';

/** Depois que a skill existe, uma falha ao salvar os textos é aviso de formulário, não erro de nome. */
type Estado = { tipo: 'formulario' } | { tipo: 'textosFalharam'; nome: string };

/** Diálogo de criação: nome, resumo e critério de reconhecimento, enviados juntos. */
export function NovaSkillDialog({
  onCriar,
  onSalvarTextos,
  onConcluida,
  onFecharComFalha,
  onCancelar,
}: Props): ReactNode {
  const { control, handleSubmit, setError, getValues, formState } = useForm({
    resolver: zodResolver(NovaSkillSchema),
    defaultValues: { titulo: '', resumo: '', quandoUsar: '' },
  });
  const identificador = paraIdentificador(useWatch({ control, name: 'titulo' }));
  const resumo = useWatch({ control, name: 'resumo' });
  const [estado, setEstado] = useState<Estado>({ tipo: 'formulario' });
  const [retentando, setRetentando] = useState(false);
  const textosFalharam = estado.tipo === 'textosFalharam';

  const textosDigitados = (): TextosDaNova => {
    const { resumo: resumoDigitado, quandoUsar } = getValues();
    return { resumo: resumoDigitado, quandoUsar };
  };

  const enviar = handleSubmit(async ({ titulo, resumo: resumoEnviado, quandoUsar }) => {
    try {
      const criada = await onCriar({ nome: paraIdentificador(titulo), titulo: titulo.trim() });
      try {
        await onSalvarTextos(criada.nome, { resumo: resumoEnviado, quandoUsar });
      } catch {
        setEstado({ tipo: 'textosFalharam', nome: criada.nome });
        return;
      }
      onConcluida(criada.nome);
    } catch (causa) {
      setError('titulo', { message: mensagemDe(causa) });
    }
  });

  const tentarDeNovo = async (nome: string) => {
    setRetentando(true);
    try {
      await onSalvarTextos(nome, textosDigitados());
      onConcluida(nome);
    } catch {
      // mesmo estado, mesma mensagem: a pessoa pode tentar de novo.
    } finally {
      setRetentando(false);
    }
  };

  const aoSubmeter = (evento: FormEvent<HTMLFormElement>) => {
    evento.preventDefault();
    if (estado.tipo === 'textosFalharam') {
      void tentarDeNovo(estado.nome);
      return;
    }
    void enviar();
  };

  const fechar = () => {
    if (estado.tipo === 'textosFalharam') {
      onFecharComFalha(estado.nome, textosDigitados());
      return;
    }
    onCancelar();
  };

  return (
    <Dialog open fullWidth maxWidth="xs" onClose={fechar}>
      <form onSubmit={aoSubmeter}>
        <DialogTitle>Nova skill</DialogTitle>
        <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          {textosFalharam && <Alert severity="error">{MENSAGEM_TEXTOS_FALHARAM}</Alert>}
          <CampoTexto
            control={control}
            name="titulo"
            autoFocus
            fullWidth
            margin="dense"
            label="Nome"
            disabled={textosFalharam}
          />
          <Typography variant="caption" color="text.secondary">
            Identificador: {identificador}
          </Typography>
          <CampoTexto
            control={control}
            name="resumo"
            fullWidth
            margin="dense"
            label="Resumo"
            helperText={`${resumo.length}/${LIMITE_DO_RESUMO}`}
            slotProps={{ htmlInput: { maxLength: LIMITE_DO_RESUMO } }}
          />
          <CampoTexto
            control={control}
            name="quandoUsar"
            multiline
            minRows={2}
            fullWidth
            margin="dense"
            label="Como reconhecer o pedido"
          />
        </DialogContent>
        <DialogActions>
          <Button onClick={fechar}>{textosFalharam ? 'Fechar' : 'Cancelar'}</Button>
          <Button
            type="submit"
            variant="contained"
            disabled={
              (!textosFalharam && identificador === '') || formState.isSubmitting || retentando
            }
          >
            {textosFalharam ? 'Tentar de novo' : 'Criar'}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
