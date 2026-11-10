import { type Eu, pode } from '@/acesso/eu';

/**
 * Gestor de administradora abre direto nos condomínios; quem só tem um condomínio abre nele;
 * o resto abre no Concierge.
 */
export function rotaInicial(eu: Eu): string {
  const ehGestorDeAdministradora =
    eu.administradoraId !== null && pode(eu, 'condominios.habilitar');
  if (!ehGestorDeAdministradora) return '/concierge';
  return eu.condominioId ? `/condominios/${encodeURIComponent(eu.condominioId)}` : '/condominios';
}
