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

export function SelecaoDeInformacoes({
  rotulo,
  valor,
  informacoes,
  editavel,
  onMudar,
}: SelecaoProps): ReactNode {
  const titulo = (nome: string): string => informacoes.find((i) => i.nome === nome)?.titulo || nome;
  const semCadastro = valor.filter((nome) => !informacoes.some((i) => i.nome === nome));
  const nomes = [...informacoes.map((i) => i.nome), ...semCadastro];
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
        onChange={(evento) => {
          const escolhidos = evento.target.value;
          onMudar(typeof escolhidos === 'string' ? escolhidos.split(',') : escolhidos);
        }}
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
