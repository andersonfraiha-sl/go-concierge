import { Button, Typography } from '@mui/material';

/** 401 não chega aqui: o StudioLayout troca a tela inteira por UsuarioNaoReconhecido. */
export function ErroAcesso({ aoTentar }: { aoTentar: () => void }) {
  return (
    <>
      <Typography role="alert">Não foi possível carregar seu acesso.</Typography>
      <Button size="small" onClick={aoTentar}>
        Tentar de novo
      </Button>
    </>
  );
}
