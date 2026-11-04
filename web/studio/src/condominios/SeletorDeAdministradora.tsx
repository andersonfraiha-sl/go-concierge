import { TextField } from '@mui/material';
import type { ReactNode } from 'react';
import type { Administradora } from './contratos';

type SeletorProps = {
  administradoras: Administradora[];
  valor: string;
  onMudar: (id: string) => void;
  desabilitado?: boolean;
};

export function SeletorDeAdministradora({
  administradoras,
  valor,
  onMudar,
  desabilitado = false,
}: SeletorProps): ReactNode {
  return (
    <TextField
      select
      label="Administradora"
      value={valor}
      disabled={desabilitado}
      onChange={(e) => onMudar(e.target.value)}
      slotProps={{ select: { native: true }, inputLabel: { shrink: true } }}
    >
      <option value="">Sem administradora</option>
      <OpcoesDeAdministradora administradoras={administradoras} />
    </TextField>
  );
}

export function OpcoesDeAdministradora({
  administradoras,
}: Pick<SeletorProps, 'administradoras'>): ReactNode {
  return administradoras.map((a) => (
    <option key={a.id} value={a.id}>
      {a.nome}
    </option>
  ));
}
