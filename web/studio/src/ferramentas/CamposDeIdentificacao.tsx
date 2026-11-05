import { Box, FormControlLabel, MenuItem, Switch } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { PESOS, RAIO, REGISTRA_FORTE } from '@/theme/tokens';
import { TIPOS } from './contratos';
import type { Valores } from './formularioDaFerramenta';

const VISIBILIDADES = { private: 'Só morador identificado', public: 'Qualquer contato' };

/** O tipo gravado fica na lista mesmo quando o Studio não o conhece. */
function tiposComAtual(atual: string): [string, string][] {
  const entradas = Object.entries(TIPOS);
  return atual in TIPOS ? entradas : [...entradas, [atual, atual]];
}

type CamposProps = { control: Control<Valores>; travado: boolean };

type SelecaoProps = CamposProps & {
  name: 'recurso.kind' | 'recurso.visibility';
  label: string;
  opcoes: readonly (readonly [string, string])[];
};

function Selecao({ control, travado, name, label, opcoes }: SelecaoProps): ReactNode {
  return (
    <CampoTexto control={control} name={name} select label={label} disabled={travado}>
      {opcoes.map(([valor, rotulo]) => (
        <MenuItem key={valor} value={valor}>
          {rotulo}
        </MenuItem>
      ))}
    </CampoTexto>
  );
}

function GravaNoSistema({ control, travado }: CamposProps): ReactNode {
  return (
    <Controller
      control={control}
      name="recurso.write"
      render={({ field }) => (
        <FormControlLabel
          label="Grava no sistema"
          sx={{
            m: 0,
            px: 1.625,
            py: 1.25,
            borderRadius: RAIO.menu,
            bgcolor: 'studio.registra.soft',
            '& .MuiFormControlLabel-label': { color: REGISTRA_FORTE, fontWeight: PESOS.semi },
          }}
          control={
            <Switch
              checked={field.value}
              disabled={travado}
              onChange={(evento) => field.onChange(evento.target.checked)}
            />
          }
        />
      )}
    />
  );
}

type Props = CamposProps & { tipoInicial: string };

export function CamposDeIdentificacao({ control, travado, tipoInicial }: Props): ReactNode {
  return (
    <>
      <CampoTexto control={control} name="recurso.titulo" label="Título" disabled={travado} />
      <CampoTexto
        control={control}
        name="recurso.name"
        label="Código"
        disabled={travado}
        helperText="Identificador usado pelas skills, como reservas.criar."
      />
      <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))', gap: 2 }}>
        <Selecao
          control={control}
          travado={travado}
          name="recurso.kind"
          label="Tipo"
          opcoes={tiposComAtual(tipoInicial)}
        />
        <Selecao
          control={control}
          travado={travado}
          name="recurso.visibility"
          label="Visibilidade"
          opcoes={Object.entries(VISIBILIDADES)}
        />
      </Box>
      <GravaNoSistema control={control} travado={travado} />
      <CampoTexto
        control={control}
        name="recurso.description"
        label="Descrição"
        disabled={travado}
        multiline
        minRows={2}
      />
    </>
  );
}
