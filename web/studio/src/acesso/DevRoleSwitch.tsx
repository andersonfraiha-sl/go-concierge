import { FormControl, InputLabel, MenuItem, Select } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { CHAVE_USUARIO_DEV } from '@/shared/api/http';

const USUARIOS_DEV = [
  { valor: 'admin-superlogica', rotulo: 'Superlógica (admin)' },
  { valor: 'construtor-superlogica', rotulo: 'Superlógica (construtor)' },
  { valor: 'gestor-exemplo', rotulo: 'Administradora (gestor)' },
  { valor: 'gestor-avancado', rotulo: 'Administradora com Avançado' },
] as const;

type UsuarioDev = (typeof USUARIOS_DEV)[number]['valor'];

function ehUsuarioDev(valor: string): valor is UsuarioDev {
  return USUARIOS_DEV.some((usuario) => usuario.valor === valor);
}

function gravarUsuario(usuario: UsuarioDev): void {
  try {
    localStorage.setItem(CHAVE_USUARIO_DEV, usuario);
  } catch (cause) {
    console.warn('gravar usuário de desenvolvimento:', cause);
  }
}

export function DevRoleSwitch({ usuarioAtual }: { usuarioAtual: string }) {
  const cliente = useQueryClient();

  const trocar = (novo: string) => {
    if (!ehUsuarioDev(novo)) return;
    gravarUsuario(novo);
    void cliente.invalidateQueries();
  };

  return (
    <FormControl size="small" sx={{ minWidth: 240 }}>
      <InputLabel id="ver-como-rotulo">Ver como</InputLabel>
      <Select
        labelId="ver-como-rotulo"
        label="Ver como"
        value={ehUsuarioDev(usuarioAtual) ? usuarioAtual : ''}
        onChange={(evento) => trocar(evento.target.value)}
      >
        {USUARIOS_DEV.map(({ valor, rotulo }) => (
          <MenuItem key={valor} value={valor}>
            {rotulo}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
