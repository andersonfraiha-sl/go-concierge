import { FormControlLabel, Switch, TextField } from '@mui/material';
import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import type { Valor } from './contratos';

type Definicao = Ferramenta['definicoes'][number];

type Props = {
  definicao: Definicao;
  valor: Valor | undefined;
  /** O que vale quando o condomínio não decide: o padrão da administradora ou o da ferramenta. */
  herdado: Valor | undefined;
  desabilitado: boolean;
  onMudar: (valor: Valor) => void;
};

const textoDe = (valor: Valor | undefined): string => (valor === undefined || valor === null ? '' : String(valor));

export function CampoAjuste({ definicao, valor, herdado, desabilitado, onMudar }: Props): ReactNode {
  const rotulo = definicao.label || definicao.name;
  const dica = textoDe(herdado) === '' ? 'Sem padrão' : `Padrão: ${textoDe(herdado)}`;
  const comum = { label: rotulo, disabled: desabilitado, helperText: definicao.description ?? dica, fullWidth: true } as const;

  if (definicao.type === 'boolean') {
    return (
      <FormControlLabel
        label={`${rotulo} (${valor === undefined ? dica.toLowerCase() : valor ? 'sim' : 'não'})`}
        control={<Switch checked={valor === true} disabled={desabilitado} onChange={(_, ligado) => onMudar(ligado)} />}
      />
    );
  }
  if (definicao.type === 'select') {
    return (
      <TextField
        {...comum}
        select
        value={textoDe(valor)}
        onChange={(e) => onMudar(e.target.value)}
        slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
      >
        <option value="">{dica}</option>
        {definicao.options.map((opcao) => (
          <option key={opcao} value={opcao}>
            {opcao}
          </option>
        ))}
      </TextField>
    );
  }
  if (definicao.type === 'number') {
    return (
      <TextField
        {...comum}
        type="number"
        value={textoDe(valor)}
        onChange={(e) => onMudar(e.target.value === '' ? '' : Number(e.target.value))}
      />
    );
  }
  return (
    <TextField
      {...comum}
      multiline={definicao.type === 'text'}
      minRows={definicao.type === 'text' ? 2 : 1}
      value={textoDe(valor)}
      onChange={(e) => onMudar(e.target.value)}
    />
  );
}
