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
import { type Control, useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import {
  LIMITE_DO_RESUMO,
  paraIdentificador,
  ResumoSchema,
  TituloDaSkillSchema,
} from './contratos';

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
  titulo: TituloDaSkillSchema,
  resumo: ResumoSchema,
  quandoUsar: z.string(),
});

const MENSAGEM_TEXTOS_FALHARAM = 'A skill foi criada, mas os textos não salvaram. Tente de novo.';

/** Depois que a skill existe, uma falha ao salvar os textos é aviso de formulário, não erro de nome. */
type Estado = { tipo: 'formulario' } | { tipo: 'textosFalharam'; nome: string };

type Valores = z.infer<typeof NovaSkillSchema>;
type Servicos = Pick<Props, 'onCriar' | 'onSalvarTextos' | 'onConcluida'>;

/** Cria a skill e salva os textos; null quando tudo deu certo e a pessoa já seguiu para o editor. */
async function criarComTextos(servicos: Servicos, valores: Valores): Promise<Estado | null> {
  const { titulo, resumo, quandoUsar } = valores;
  const criada = await servicos.onCriar({ nome: paraIdentificador(titulo), titulo: titulo.trim() });
  try {
    await servicos.onSalvarTextos(criada.nome, { resumo, quandoUsar });
  } catch {
    return { tipo: 'textosFalharam', nome: criada.nome };
  }
  servicos.onConcluida(criada.nome);
  return null;
}

function useFormDaNova() {
  return useForm({
    resolver: zodResolver(NovaSkillSchema),
    defaultValues: { titulo: '', resumo: '', quandoUsar: '' },
  });
}

function useNovaSkill(props: Props) {
  const { control, handleSubmit, setError, getValues, formState } = useFormDaNova();
  const identificador = paraIdentificador(useWatch({ control, name: 'titulo' }));
  const [estado, setEstado] = useState<Estado>({ tipo: 'formulario' });
  const [retentando, setRetentando] = useState(false);

  const textosDigitados = (): TextosDaNova => {
    const { resumo, quandoUsar } = getValues();
    return { resumo, quandoUsar };
  };

  const enviar = handleSubmit(async (valores) => {
    try {
      const proximo = await criarComTextos(props, valores);
      if (proximo !== null) setEstado(proximo);
    } catch (causa) {
      setError('titulo', { message: mensagemDe(causa) });
    }
  });

  const tentarDeNovo = async (nome: string) => {
    setRetentando(true);
    try {
      await props.onSalvarTextos(nome, textosDigitados());
      props.onConcluida(nome);
    } catch {
      // mesmo estado, mesma mensagem: a pessoa pode tentar de novo.
    } finally {
      setRetentando(false);
    }
  };

  const aoSubmeter = (evento: FormEvent<HTMLFormElement>) => {
    evento.preventDefault();
    if (estado.tipo === 'textosFalharam') void tentarDeNovo(estado.nome);
    else void enviar();
  };

  const fechar = () => {
    if (estado.tipo === 'textosFalharam') props.onFecharComFalha(estado.nome, textosDigitados());
    else props.onCancelar();
  };

  const textosFalharam = estado.tipo === 'textosFalharam';
  const envioDesabilitado =
    (!textosFalharam && identificador === '') || formState.isSubmitting || retentando;
  return { control, identificador, textosFalharam, envioDesabilitado, aoSubmeter, fechar };
}

/** Diálogo de criação: nome, resumo e critério de reconhecimento, enviados juntos. */
export function NovaSkillDialog(props: Props): ReactNode {
  const nova = useNovaSkill(props);
  return (
    <Dialog open fullWidth maxWidth="xs" onClose={nova.fechar}>
      <form onSubmit={nova.aoSubmeter}>
        <DialogTitle>Nova skill</DialogTitle>
        <CamposDaNovaSkill
          control={nova.control}
          identificador={nova.identificador}
          textosFalharam={nova.textosFalharam}
        />
        <DialogActions>
          <Button onClick={nova.fechar}>{nova.textosFalharam ? 'Fechar' : 'Cancelar'}</Button>
          <Button type="submit" variant="contained" disabled={nova.envioDesabilitado}>
            {nova.textosFalharam ? 'Tentar de novo' : 'Criar'}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

type PropsDosCampos = {
  control: Control<Valores>;
  identificador: string;
  textosFalharam: boolean;
};

function CamposDaNovaSkill({ control, identificador, textosFalharam }: PropsDosCampos): ReactNode {
  return (
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
      <CampoDoResumo control={control} />
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
  );
}

function CampoDoResumo({ control }: { control: Control<Valores> }): ReactNode {
  const resumo = useWatch({ control, name: 'resumo' });
  return (
    <CampoTexto
      control={control}
      name="resumo"
      fullWidth
      margin="dense"
      label="Resumo"
      helperText={`${resumo.length}/${LIMITE_DO_RESUMO}`}
      slotProps={{ htmlInput: { maxLength: LIMITE_DO_RESUMO } }}
    />
  );
}
