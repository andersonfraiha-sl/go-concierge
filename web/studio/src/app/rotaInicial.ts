import { type Eu, pode } from '@/acesso/eu';

/** Gestor de administradora abre direto nos condomínios; o resto abre nas skills. */
export function rotaInicial(eu: Eu): '/condominios' | '/skills' {
  const ehGestorDeAdministradora =
    eu.administradoraId !== null && pode(eu, 'condominios.habilitar');
  return ehGestorDeAdministradora ? '/condominios' : '/skills';
}
