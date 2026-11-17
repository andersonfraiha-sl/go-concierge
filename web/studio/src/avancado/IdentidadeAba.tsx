import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button, FormControlLabel, MenuItem, Radio, RadioGroup } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { type Control, Controller, useForm } from 'react-hook-form';
import { z } from 'zod';
import { chaveDoAgente, salvarConfigDoAgente, useAgente } from '@/catalogo/contratos';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { SectionCard } from '@/shared/ui/SectionCard';
import { chaveDosTextos, salvarTextos, type TextosLidos, useTextos } from '@/skills/contratos';
import { mensagemDeErro, type NoDaArvore } from './contratos';

const TIPOS = ['', 'orquestrador', 'operador'] as const;
type TipoDeclarado = (typeof TIPOS)[number];

const ROTULO_DO_TIPO: Record<TipoDeclarado, string> = {
  '': 'Automático',
  orquestrador: 'Orquestrador',
  operador: 'Operador',
};

const isTipoDeclarado = (valor: string): valor is TipoDeclarado =>
  TIPOS.some((tipo) => tipo === valor);

const VISIBILIDADES = ['private', 'public'] as const;
type Visibilidade = (typeof VISIBILIDADES)[number];

const ROTULO_DA_VISIBILIDADE: Record<Visibilidade, string> = {
  private: 'Só morador identificado',
  public: 'Qualquer contato',
};

const isVisibilidade = (valor: string): valor is Visibilidade =>
  VISIBILIDADES.some((visibilidade) => visibilidade === valor);

const ultimoSegmento = (caminho: string): string => caminho.split('/').at(-1) ?? caminho;

const RascunhoSchema = z.object({
  tipo: z.enum(TIPOS),
  modelo: z.string(),
  visibilidade: z.string(),
});
type Rascunho = z.infer<typeof RascunhoSchema>;

type Props = { no: NoDaArvore; editar: boolean };

export function IdentidadeAba({ no, editar }: Props): ReactNode {
  const agente = useAgente(no.caminho);
  const nome = ultimoSegmento(no.caminho);
  const textos = useTextos(nome, { enabled: no.caso });

  if (agente.isPending) return null;
  if (agente.isError) {
    return (
      <ErroComRetry mensagem={agente.error.message} onTentarDeNovo={() => void agente.refetch()} />
    );
  }
  if (no.caso && textos.isPending) return null;
  if (no.caso && textos.isError) {
    return (
      <ErroComRetry mensagem={textos.error.message} onTentarDeNovo={() => void textos.refetch()} />
    );
  }

  return (
    <Formulario
      agente={no.caminho}
      dados={agente.data.agente}
      textos={no.caso && textos.data ? { nome, lido: textos.data } : null}
      editar={editar}
    />
  );
}

type TextosDaSkill = { nome: string; lido: TextosLidos } | null;

type FormularioProps = {
  agente: string;
  dados: { tipo: string; tipo_declarado: boolean; modelo: string };
  textos: TextosDaSkill;
  editar: boolean;
};

const rascunhoInicial = (dados: FormularioProps['dados'], textos: TextosDaSkill): Rascunho => {
  const visibilidadeLida = textos?.lido.textos.visibilidade ?? '';
  return {
    tipo: dados.tipo_declarado && isTipoDeclarado(dados.tipo) ? dados.tipo : '',
    modelo: dados.modelo,
    visibilidade: isVisibilidade(visibilidadeLida) ? visibilidadeLida : 'private',
  };
};

type SalvarIdentidade = (rascunho: Rascunho) => Promise<void>;

function useSalvarIdentidade(agente: string, textos: TextosDaSkill): SalvarIdentidade {
  const { mostrar } = useSnackbar();
  const cliente = useQueryClient();
  return async (rascunho) => {
    try {
      await salvarConfigDoAgente({
        agente,
        campos: { tipo: rascunho.tipo, modelo: rascunho.modelo },
      });
      await cliente.invalidateQueries({ queryKey: chaveDoAgente(agente) });
      if (textos) {
        await salvarTextos(textos.nome, {
          versao: textos.lido.versao,
          textos: { ...textos.lido.textos, visibilidade: rascunho.visibilidade },
        });
        await cliente.invalidateQueries({ queryKey: chaveDosTextos(textos.nome) });
      }
      mostrar('Identidade salva');
    } catch (erro) {
      mostrar(mensagemDeErro(erro), 'error');
    }
  };
}

function Formulario({ agente, dados, textos, editar }: FormularioProps): ReactNode {
  const salvarIdentidade = useSalvarIdentidade(agente, textos);
  const { control, handleSubmit, formState } = useForm({
    resolver: zodResolver(RascunhoSchema),
    defaultValues: rascunhoInicial(dados, textos),
  });
  const salvar = handleSubmit(salvarIdentidade);

  return (
    <SectionCard
      titulo="Identidade"
      ajuda="O tipo técnico, o modelo e quem pode falar com esta skill."
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
      <CamposTecnicos control={control} editar={editar} />
      {textos && <SeletorDeVisibilidade control={control} editar={editar} />}
    </SectionCard>
  );
}

type CamposProps = { control: Control<Rascunho>; editar: boolean };

function CamposTecnicos({ control, editar }: CamposProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
      <CampoTexto
        control={control}
        name="tipo"
        select
        label="Tipo"
        size="small"
        disabled={!editar}
        sx={{ minWidth: 220 }}
      >
        {TIPOS.map((tipo) => (
          <MenuItem key={tipo} value={tipo}>
            {ROTULO_DO_TIPO[tipo]}
          </MenuItem>
        ))}
      </CampoTexto>
      <CampoTexto
        control={control}
        name="modelo"
        label="Modelo"
        size="small"
        disabled={!editar}
        helperText="Vazio usa o modelo padrão."
        sx={{ minWidth: 260 }}
      />
    </Box>
  );
}

function SeletorDeVisibilidade({ control, editar }: CamposProps): ReactNode {
  return (
    <Controller
      control={control}
      name="visibilidade"
      render={({ field }) => (
        <RadioGroup
          aria-label="Visibilidade"
          value={field.value}
          onChange={(_, valor) => {
            if (isVisibilidade(valor)) field.onChange(valor);
          }}
        >
          {VISIBILIDADES.map((visibilidade) => (
            <FormControlLabel
              key={visibilidade}
              value={visibilidade}
              disabled={!editar}
              control={<Radio />}
              label={ROTULO_DA_VISIBILIDADE[visibilidade]}
            />
          ))}
        </RadioGroup>
      )}
    />
  );
}
