import { Box, Button, FormControlLabel, InputAdornment, Switch, TextField } from '@mui/material';
import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { separarUnidade } from './ajustes';
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

const textoDe = (valor: Valor | undefined): string =>
  valor === undefined || valor === null ? '' : String(valor);

const INPUT_LABEL_FLUTUANTE = { inputLabel: { shrink: true } } as const;

export function CampoAjuste({
  definicao,
  valor,
  herdado,
  desabilitado,
  onMudar,
}: Props): ReactNode {
  const { rotulo, unidade } = separarUnidade(definicao.label || definicao.name);
  const textoHerdado = textoDe(herdado);
  const placeholder = textoHerdado === '' ? 'Sem padrão' : textoHerdado;
  const temDecisao = valor !== undefined && valor !== '';

  const usarOPadrao = (
    <Button size="small" aria-label={`Usar o padrão: ${rotulo}`} onClick={() => onMudar('')}>
      Usar o padrão
    </Button>
  );

  const comum = {
    label: rotulo,
    disabled: desabilitado,
    helperText: definicao.description,
    placeholder,
    fullWidth: true,
  } as const;

  if (definicao.type === 'boolean') {
    const padraoLigado = herdado === true;
    const situacao = temDecisao
      ? valor
        ? 'sim'
        : 'não'
      : `padrão: ${padraoLigado ? 'sim' : 'não'}`;
    return (
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <FormControlLabel
          label={`${rotulo} (${situacao})`}
          control={
            <Switch
              checked={valor === true}
              disabled={desabilitado}
              onChange={(_, ligado) => onMudar(ligado)}
            />
          }
        />
        {temDecisao && !desabilitado && usarOPadrao}
      </Box>
    );
  }

  if (definicao.type === 'select') {
    return (
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
        <TextField
          {...comum}
          select
          value={textoDe(valor)}
          onChange={(e) => onMudar(e.target.value)}
          slotProps={{ ...INPUT_LABEL_FLUTUANTE, select: { native: true } }}
        >
          <option value="">{textoHerdado === '' ? 'Sem padrão' : `Padrão: ${textoHerdado}`}</option>
          {definicao.options.map((opcao) => (
            <option key={opcao} value={opcao}>
              {opcao}
            </option>
          ))}
        </TextField>
        {temDecisao && !desabilitado && usarOPadrao}
      </Box>
    );
  }

  if (definicao.type === 'number') {
    return (
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
        <TextField
          {...comum}
          type="number"
          value={textoDe(valor)}
          onChange={(e) => onMudar(e.target.value === '' ? '' : Number(e.target.value))}
          slotProps={{
            ...INPUT_LABEL_FLUTUANTE,
            ...(unidade !== null && {
              input: { endAdornment: <InputAdornment position="end">{unidade}</InputAdornment> },
            }),
          }}
        />
        {temDecisao && !desabilitado && usarOPadrao}
      </Box>
    );
  }

  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
      <TextField
        {...comum}
        multiline={definicao.type === 'text'}
        minRows={definicao.type === 'text' ? 2 : 1}
        value={textoDe(valor)}
        onChange={(e) => onMudar(e.target.value)}
        slotProps={INPUT_LABEL_FLUTUANTE}
      />
      {temDecisao && !desabilitado && usarOPadrao}
    </Box>
  );
}
