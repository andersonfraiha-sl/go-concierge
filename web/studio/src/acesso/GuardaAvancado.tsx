import { AvancadoPage } from '@/avancado/AvancadoPage';
import { ErroAcesso } from './ErroAcesso';
import { pode, useEu } from './eu';
import { SemAcesso } from './SemAcesso';

export function GuardaAvancado() {
  const { data: eu, isPending, isError, error, refetch } = useEu();
  if (isPending) return null;
  if (isError) return <ErroAcesso erro={error} aoTentar={() => void refetch()} />;
  return pode(eu, 'studio.avancado') ? <AvancadoPage /> : <SemAcesso />;
}
