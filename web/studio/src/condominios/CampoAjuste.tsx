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

function propsComuns({ definicao, herdado, desabilitado }: Props) {
  const { rotulo, unidade } = separarUnidade(definicao.label || definicao.name);
  const textoHerdado = textoDe(herdado);
  const comum = {
    label: rotulo,
    disabled: desabilitado,
    helperText: definicao.description,
    placeholder: textoHerdado === '' ? 'Sem padrão' : textoHerdado,
    fullWidth: true,
  } as const;
  return { rotulo, unidade, textoHerdado, comum };
}

export function CampoAjuste(props: Props): ReactNode {
  const { definicao, valor, desabilitado, onMudar } = props;
  const { rotulo } = propsComuns(props);
  const temDecisao = valor !== undefined && valor !== '';
  const alinhamento = definicao.type === 'boolean' ? 'center' : 'flex-start';

  return (
    <Box sx={{ display: 'flex', alignItems: alinhamento, gap: 1 }}>
      <EntradaDoCampo {...props} />
      {temDecisao && !desabilitado && (
        <Button size="small" aria-label={`Usar o padrão: ${rotulo}`} onClick={() => onMudar('')}>
          Usar o padrão
        </Button>
      )}
    </Box>
  );
}

function EntradaDoCampo(props: Props): ReactNode {
  if (props.definicao.type === 'boolean') return <EntradaLiga {...props} />;
  if (props.definicao.type === 'select') return <EntradaEscolha {...props} />;
  if (props.definicao.type === 'number') return <EntradaNumero {...props} />;
  return <EntradaTexto {...props} />;
}

function EntradaLiga(props: Props): ReactNode {
  const { valor, herdado, desabilitado, onMudar } = props;
  const { rotulo } = propsComuns(props);
  const temDecisao = valor !== undefined && valor !== '';
  const padraoLigado = herdado === true;
  const decidido = valor ? 'sim' : 'não';
  const situacao = temDecisao ? decidido : `padrão: ${padraoLigado ? 'sim' : 'não'}`;
  const interruptor = (
    <Switch
      checked={valor === true}
      disabled={desabilitado}
      onChange={(_, ligado) => onMudar(ligado)}
    />
  );
  return <FormControlLabel label={`${rotulo} (${situacao})`} control={interruptor} />;
}

function EntradaEscolha(props: Props): ReactNode {
  const { definicao, valor, onMudar } = props;
  const { comum, textoHerdado } = propsComuns(props);
  return (
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
  );
}

function EntradaNumero(props: Props): ReactNode {
  const { valor, onMudar } = props;
  const { comum, unidade } = propsComuns(props);
  const adorno = unidade !== null && {
    input: { endAdornment: <InputAdornment position="end">{unidade}</InputAdornment> },
  };
  return (
    <TextField
      {...comum}
      type="number"
      value={textoDe(valor)}
      onChange={(e) => onMudar(e.target.value === '' ? '' : Number(e.target.value))}
      slotProps={{ ...INPUT_LABEL_FLUTUANTE, ...adorno }}
    />
  );
}

function EntradaTexto(props: Props): ReactNode {
  const { definicao, valor, onMudar } = props;
  const { comum } = propsComuns(props);
  const longo = definicao.type === 'text';
  return (
    <TextField
      {...comum}
      multiline={longo}
      minRows={longo ? 2 : 1}
      value={textoDe(valor)}
      onChange={(e) => onMudar(e.target.value)}
      slotProps={INPUT_LABEL_FLUTUANTE}
    />
  );
}
