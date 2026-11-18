import { type Eu, pode } from '@/acesso/eu';

export function rotaInicial(eu: Eu): string {
  const ehGestorDeAdministradora =
    eu.administradoraId !== null && pode(eu, 'condominios.habilitar');
  if (!ehGestorDeAdministradora) return '/concierge';
  const company = encodeURIComponent(eu.administradoraId ?? '');
  const caminho = eu.condominioId
    ? `/condominios/${encodeURIComponent(eu.condominioId)}`
    : '/condominios';
  return `${caminho}?company_id=${company}`;
}
