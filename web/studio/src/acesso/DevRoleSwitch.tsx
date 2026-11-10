import { FormControl, InputLabel, MenuItem, Select } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { gravarUsuarioDev } from './usuarioDev';

const USUARIOS_DEV = [
  { valor: 'admin-superlogica', rotulo: 'Superlógica (admin)' },
  { valor: 'construtor-superlogica', rotulo: 'Superlógica (construtor)' },
  { valor: 'gestor-exemplo', rotulo: 'Administradora (gestor)' },
  { valor: 'gestor-avancado', rotulo: 'Administradora com Avançado' },
  { valor: 'gestor-um-condominio', rotulo: 'Administradora de um condomínio' },
  { valor: 'gestor-condominio', rotulo: 'Só um condomínio' },
] as const;

type UsuarioDev = (typeof USUARIOS_DEV)[number]['valor'];

function ehUsuarioDev(valor: string): valor is UsuarioDev {
  return USUARIOS_DEV.some((usuario) => usuario.valor === valor);
}

const OPCOES_DE_USUARIO = USUARIOS_DEV.map(({ valor, rotulo }) => (
  <MenuItem key={valor} value={valor}>
    {rotulo}
  </MenuItem>
));

function useTrocarUsuarioDev(): (novo: string) => void {
  const cliente = useQueryClient();
  return (novo) => {
    if (!ehUsuarioDev(novo)) return;
    // Tudo no mesmo clique: o Eu muda de chave no próximo render, então ninguém lê o anterior.
    // O Eu antigo sai do cache em vez de ser rebuscado com o cabeçalho do usuário novo.
    gravarUsuarioDev(novo);
    cliente.removeQueries({ queryKey: ['eu'] });
    void cliente.resetQueries();
  };
}

export function DevRoleSwitch({ usuarioAtual }: { usuarioAtual: string }) {
  const trocar = useTrocarUsuarioDev();
  const valor = ehUsuarioDev(usuarioAtual) ? usuarioAtual : '';

  return (
    <FormControl size="small" sx={{ minWidth: 240 }}>
      <InputLabel id="ver-como-rotulo">Ver como</InputLabel>
      <Select
        labelId="ver-como-rotulo"
        label="Ver como"
        value={valor}
        onChange={(evento) => trocar(evento.target.value)}
      >
        {OPCOES_DE_USUARIO}
      </Select>
    </FormControl>
  );
}
