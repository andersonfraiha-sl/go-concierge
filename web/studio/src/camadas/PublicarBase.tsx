import { Button } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import { PublicarBaseDialog } from './PublicarBaseDialog';

/** Botão do cabeçalho da skill; só quem publica a base (fora de administradora) o vê. */
export function PublicarBase({ skill }: { skill: string }): ReactNode {
  const [aberto, setAberto] = useState(false);
  const podePublicar = usePodeEditar('skills.publicar_base');
  if (!podePublicar) return null;
  return (
    <>
      <Button variant="contained" onClick={() => setAberto(true)}>
        Publicar versão
      </Button>
      <PublicarBaseDialog skill={skill} aberto={aberto} onFechar={() => setAberto(false)} />
    </>
  );
}
