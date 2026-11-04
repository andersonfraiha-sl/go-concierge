import { FormControl, InputLabel, MenuItem, OutlinedInput, Select } from '@mui/material';
import { type ReactNode, useId } from 'react';
import type { Informacao } from '@/fluxo/modelo/tipos';

type SelecaoProps = {
  rotulo: string;
  valor: readonly string[];
  informacoes: readonly Informacao[];
  editavel: boolean;
  onMudar: (nomes: string[]) => void;
};

const tituloDaInformacao = (informacoes: readonly Informacao[], nome: string): string =>
  informacoes.find((informacao) => informacao.nome === nome)?.titulo || nome;

/** O MUI entrega string quando o autofill preenche o campo múltiplo. */
const emLista = (escolhidos: string | string[]): string[] =>
  typeof escolhidos === 'string' ? escolhidos.split(',') : escolhidos;

export function SelecaoDeInformacoes({
  rotulo,
  valor,
  informacoes,
  editavel,
  onMudar,
}: SelecaoProps): ReactNode {
  const titulo = (nome: string) => tituloDaInformacao(informacoes, nome);
  const semCadastro = valor.filter(
    (nome) => !informacoes.some((informacao) => informacao.nome === nome),
  );
  const nomes = [...informacoes.map((informacao) => informacao.nome), ...semCadastro];
  const id = useId();
  return (
    <FormControl size="small" fullWidth disabled={!editavel}>
      <InputLabel id={id}>{rotulo}</InputLabel>
      <Select<string[]>
        multiple
        labelId={id}
        value={[...valor]}
        input={<OutlinedInput label={rotulo} />}
        renderValue={(escolhidos) => escolhidos.map(titulo).join(', ')}
        onChange={(evento) => onMudar(emLista(evento.target.value))}
      >
        {nomes.map((nome) => (
          <MenuItem key={nome} value={nome}>
            {titulo(nome)}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
