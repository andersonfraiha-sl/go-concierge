import { Button, Typography } from '@mui/material';
import type { ApiError } from '@/shared/api/http';

const STATUS_NAO_AUTENTICADO = 401;

export function ErroAcesso({ erro, aoTentar }: { erro: ApiError | null; aoTentar: () => void }) {
  if (erro?.status === STATUS_NAO_AUTENTICADO) {
    return (
      <>
        <Typography role="alert">Não reconhecemos seu usuário.</Typography>
        <Typography variant="body2" color="text.secondary">
          Entre de novo para continuar.
        </Typography>
      </>
    );
  }
  return (
    <>
      <Typography role="alert">Não foi possível carregar seu acesso.</Typography>
      <Button size="small" onClick={aoTentar}>
        Tentar de novo
      </Button>
    </>
  );
}
