import { FormControlLabel, MenuItem, Switch } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { TIPOS } from './contratos';
import type { Valores } from './formularioDaFerramenta';

const VISIBILIDADES = { private: 'Só morador identificado', public: 'Qualquer contato' };

/** O tipo gravado fica na lista mesmo quando o Studio não o conhece. */
function tiposComAtual(atual: string): [string, string][] {
  const entradas = Object.entries(TIPOS);
  return atual in TIPOS ? entradas : [...entradas, [atual, atual]];
}

type Props = { control: Control<Valores>; travado: boolean; tipoInicial: string };

export function CamposDeIdentificacao({ control, travado, tipoInicial }: Props): ReactNode {
  return (
    <>
      <CampoTexto control={control} name="recurso.name" label="Nome" disabled={travado} />
      <CampoTexto control={control} name="recurso.titulo" label="Título" disabled={travado} />
      <CampoTexto control={control} name="recurso.kind" select label="Tipo" disabled={travado}>
        {tiposComAtual(tipoInicial).map(([valor, rotulo]) => (
          <MenuItem key={valor} value={valor}>
            {rotulo}
          </MenuItem>
        ))}
      </CampoTexto>
      <CampoTexto
        control={control}
        name="recurso.visibility"
        select
        label="Visibilidade"
        disabled={travado}
      >
        {Object.entries(VISIBILIDADES).map(([valor, rotulo]) => (
          <MenuItem key={valor} value={valor}>
            {rotulo}
          </MenuItem>
        ))}
      </CampoTexto>
      <Controller
        control={control}
        name="recurso.write"
        render={({ field }) => (
          <FormControlLabel
            label="Grava no sistema"
            control={
              <Switch
                checked={field.value}
                disabled={travado}
                onChange={(e) => field.onChange(e.target.checked)}
              />
            }
          />
        )}
      />
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
