import { FormControl, InputLabel, MenuItem, Select } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { CHAVE_PAPEL_DEV } from '@/shared/api/http';

const PAPEIS_DEV = [
  { valor: 'construtor', rotulo: 'Superlógica (construtor)' },
  { valor: 'gestor', rotulo: 'Administradora (gestor)' },
  { valor: 'gestor_avancado', rotulo: 'Administradora com Avançado' },
] as const;

type PapelDev = (typeof PAPEIS_DEV)[number]['valor'];

function ehPapelDev(valor: string): valor is PapelDev {
  return PAPEIS_DEV.some((papel) => papel.valor === valor);
}

function gravarPapel(papel: PapelDev): void {
  try {
    localStorage.setItem(CHAVE_PAPEL_DEV, papel);
  } catch (cause) {
    console.warn('gravar papel de desenvolvimento:', cause);
  }
}

export function DevRoleSwitch({ papelAtual }: { papelAtual: string }) {
  const cliente = useQueryClient();

  const trocar = (novo: string) => {
    if (!ehPapelDev(novo)) return;
    gravarPapel(novo);
    void cliente.invalidateQueries();
  };

  return (
    <FormControl size="small" sx={{ minWidth: 240 }}>
      <InputLabel id="ver-como-rotulo">Ver como</InputLabel>
      <Select
        labelId="ver-como-rotulo"
        label="Ver como"
        value={ehPapelDev(papelAtual) ? papelAtual : ''}
        onChange={(evento) => trocar(evento.target.value)}
      >
        {PAPEIS_DEV.map(({ valor, rotulo }) => (
          <MenuItem key={valor} value={valor}>
            {rotulo}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
