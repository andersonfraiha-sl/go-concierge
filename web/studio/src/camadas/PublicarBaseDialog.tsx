import { Button, CircularProgress, Dialog, DialogActions, DialogTitle } from '@mui/material';
import type { ReactNode } from 'react';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { usePreviaDaBase } from './contratos';
import { FormularioDePublicacao } from './FormularioDePublicacao';

type Props = { skill: string; aberto: boolean; onFechar: () => void };

const ID_DO_TITULO = 'publicar-base-titulo';

/** Publicar uma versão da base: o que mudou, quais trechos são obrigatórios e se a skill encerra. */
export function PublicarBaseDialog({ skill, aberto, onFechar }: Props): ReactNode {
  return (
    <Dialog open={aberto} onClose={onFechar} fullWidth maxWidth="md" aria-labelledby={ID_DO_TITULO}>
      {/* Montado só com o diálogo aberto: a prévia é lida na hora, nunca de uma abertura anterior. */}
      <ConteudoDoDialogo skill={skill} onFechar={onFechar} />
    </Dialog>
  );
}

function ConteudoDoDialogo({ skill, onFechar }: Omit<Props, 'aberto'>): ReactNode {
  const previa = usePreviaDaBase(skill);
  return (
    <>
      <DialogTitle id={ID_DO_TITULO}>Publicar versão</DialogTitle>
      {previa.isPending && <CircularProgress aria-label="Carregando mudanças" sx={{ m: 3 }} />}
      {previa.isError && (
        <>
          <ErroComRetry
            mensagem={previa.error.message}
            onTentarDeNovo={() => void previa.refetch()}
          />
          <DialogActions>
            <Button onClick={onFechar}>Cancelar</Button>
          </DialogActions>
        </>
      )}
      {previa.isSuccess && (
        <FormularioDePublicacao skill={skill} previa={previa.data} onFechar={onFechar} />
      )}
    </>
  );
}
