import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button, TextField, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import type { Ferramenta } from '@/catalogo/contratos';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { type Sugestao, sugerirAjustes, useEscopo, type Valor } from './contratos';

type Props = {
  condominio: string;
  ferramentas: Ferramenta[];
  valores: Record<string, Record<string, Valor>>;
  onPreencher: (recurso: string, campos: Record<string, Valor>) => void;
};

const FraseSchema = z.object({ frase: z.string().refine((frase) => frase.trim() !== '') });

function notaDaSugestao(sugestao: Sugestao): string {
  const semNada = Object.keys(sugestao.payload).length === 0;
  return (
    sugestao.nota || (semNada ? 'Não encontrei nada no texto.' : 'Revise os campos preenchidos.')
  );
}

type Pedido = Pick<Props, 'condominio' | 'valores' | 'onPreencher'> & {
  recurso: string;
  frase: string;
  escopo: string | null;
};

/** Pede a sugestão, aplica no formulário e devolve o aviso para mostrar (nota ou erro). */
async function aplicarSugestao({ recurso, frase, escopo, ...resto }: Pedido): Promise<string> {
  try {
    const atual = resto.valores[recurso] ?? {};
    const pedido = { condominio: resto.condominio, recurso, texto: frase, atual };
    const sugestao = await sugerirAjustes(pedido, escopo);
    resto.onPreencher(recurso, sugestao.payload);
    return notaDaSugestao(sugestao);
  } catch (erro) {
    return erro instanceof Error ? erro.message : String(erro);
  }
}

function useSugestao(props: Omit<Props, 'ferramentas'> & { recurso: string }) {
  const escopo = useEscopo() ?? null;
  const [aviso, setAviso] = useState<string | null>(null);
  const { control, handleSubmit, formState } = useForm({
    resolver: zodResolver(FraseSchema),
    defaultValues: { frase: '' },
  });
  const frase = useWatch({ control, name: 'frase' });

  const sugerir = handleSubmit(async (valores) => {
    setAviso(null);
    setAviso(await aplicarSugestao({ ...props, frase: valores.frase, escopo }));
  });

  return { control, frase, aviso, sugerir, pensando: formState.isSubmitting };
}

/** Botão único que abre a caixa de sugestão por IA para os ajustes gerais do condomínio. */
export function PreencherComIA(props: Props): ReactNode {
  const [aberto, setAberto] = useState(false);

  if (props.ferramentas.length === 0) return null;
  if (aberto) return <CaixaDeSugestao {...props} />;
  return (
    <Button variant="outlined" onClick={() => setAberto(true)}>
      Preencher com IA
    </Button>
  );
}

function CaixaDeSugestao(props: Props): ReactNode {
  const [recurso, setRecurso] = useState(() => props.ferramentas[0]?.name ?? '');
  const sugestao = useSugestao({ ...props, recurso });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      {props.ferramentas.length > 1 && (
        <SeletorDeFerramenta
          ferramentas={props.ferramentas}
          recurso={recurso}
          onMudar={setRecurso}
        />
      )}
      <LinhaDaFrase sugestao={sugestao} />
      <AvisoDaSugestao aviso={sugestao.aviso} />
    </Box>
  );
}

function LinhaDaFrase({ sugestao }: { sugestao: ReturnType<typeof useSugestao> }): ReactNode {
  const { control, frase, pensando, sugerir } = sugestao;
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
      <CampoTexto
        control={control}
        name="frase"
        size="small"
        fullWidth
        label="Descreva em uma frase"
      />
      <BotaoSugerir
        pensando={pensando}
        vazio={frase.trim() === ''}
        onSugerir={() => void sugerir()}
      />
    </Box>
  );
}

function AvisoDaSugestao({ aviso }: { aviso: string | null }): ReactNode {
  if (aviso === null) return null;
  return (
    <Typography variant="caption" color="textSecondary" role="status">
      {aviso}
    </Typography>
  );
}

type SeletorProps = { ferramentas: Ferramenta[]; recurso: string; onMudar: (nome: string) => void };

function SeletorDeFerramenta({ ferramentas, recurso, onMudar }: SeletorProps): ReactNode {
  return (
    <TextField
      select
      label="Ferramenta"
      size="small"
      value={recurso}
      onChange={(e) => onMudar(e.target.value)}
      slotProps={{ select: { native: true } }}
    >
      {ferramentas.map((ferramenta) => (
        <option key={ferramenta.name} value={ferramenta.name}>
          {ferramenta.titulo ?? ferramenta.name}
        </option>
      ))}
    </TextField>
  );
}

type BotaoProps = { pensando: boolean; vazio: boolean; onSugerir: () => void };

function BotaoSugerir({ pensando, vazio, onSugerir }: BotaoProps): ReactNode {
  return (
    <Button variant="outlined" disabled={pensando || vazio} onClick={onSugerir}>
      {pensando ? 'Pensando…' : 'Sugerir'}
    </Button>
  );
}
