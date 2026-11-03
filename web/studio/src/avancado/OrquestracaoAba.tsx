import { Alert, Box, Button, FormControlLabel, Radio, RadioGroup, TextField, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  LIMITES_DE_ORQUESTRACAO,
  MODOS_DE_ORQUESTRACAO,
  mensagemDeErro,
  salvarOrquestracao,
  useOrquestracao,
  validarOrquestracao,
  type ModoDeOrquestracao,
  type Orquestracao,
  type OrquestracaoLida,
} from './contratos';

const MODOS: Record<ModoDeOrquestracao, { nome: string; descricao: string }> = {
  router: { nome: 'Router', descricao: 'Escolhe uma skill por mensagem. Serve para pedidos de um item só.' },
  planning: {
    nome: 'Planning',
    descricao: 'Monta a lista de tudo o que o morador pediu e cumpre item por item. Serve para mensagens com vários pedidos.',
  },
  react: {
    nome: 'ReAct',
    descricao: 'Pensa, age e observa a cada passo. Serve para pedidos abertos, em que o próximo passo depende do anterior.',
  },
};

type Rascunho = { modo: ModoDeOrquestracao; bind_confidence: string; max_itens: string; max_iteracoes: string };

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
  const [rascunho, setRascunho] = useState(rascunhoDe(lida.arquivo));
  const [problemas, setProblemas] = useState<string[]>([]);
  const [salvando, setSalvando] = useState(false);

  const salvar = async (): Promise<void> => {
    const ajuste = ajusteDe(rascunho);
    const invalidos = validarOrquestracao(ajuste);
    setProblemas(invalidos);
    if (invalidos.length > 0) return;
    setSalvando(true);
    try {
      const gravada = await salvarOrquestracao(ajuste);
      await cliente.invalidateQueries({ queryKey: ['avancado', 'orquestracao'] });
      mostrar(avisoDeSalvo(gravada));
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    } finally {
      setSalvando(false);
    }
  };

  return (
    <SectionCard
      titulo="Orquestração"
      ajuda="Como a assistente decide o que fazer com cada mensagem."
      acao={
        editar && (
          <Button variant="contained" disabled={salvando} onClick={() => void salvar()}>
            Salvar
          </Button>
        )
      }
    >
      {lida.ambiente !== '' && (
        <Alert severity="warning">
          O ambiente está forçando o modo {lida.ambiente}. O que você salvar aqui só vale quando essa trava sair.
        </Alert>
      )}
      <RadioGroup
        aria-label="Modo"
        value={rascunho.modo}
        onChange={(_, valor) => {
          const modo = MODOS_DE_ORQUESTRACAO.find((m) => m === valor);
          if (modo) setRascunho({ ...rascunho, modo });
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
              </Box>
            }
          />
        ))}
      </RadioGroup>
      <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
        {LIMITES_DE_ORQUESTRACAO.map(({ campo, rotulo, min, max, passo }) => (
          <TextField
            key={campo}
            type="number"
            size="small"
            label={rotulo}
            value={rascunho[campo]}
            onChange={(evento) => setRascunho({ ...rascunho, [campo]: evento.target.value })}
            slotProps={{ htmlInput: { min, max, step: passo, readOnly: !editar } }}
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
