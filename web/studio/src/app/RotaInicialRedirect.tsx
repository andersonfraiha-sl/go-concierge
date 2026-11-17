import { Navigate } from 'react-router';
import { useEu } from '@/acesso/eu';
import { rotaInicial } from './rotaInicial';

export function RotaInicial() {
  const { data: eu, isPending, isError } = useEu();
  if (isPending) return null;
  if (isError) return <Navigate to="/skills" replace />;
  return <Navigate to={rotaInicial(eu)} replace />;
}
