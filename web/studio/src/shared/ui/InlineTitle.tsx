import { ButtonBase, InputBase, Typography } from '@mui/material';
import { type KeyboardEvent, type ReactNode, useRef, useState } from 'react';

type Props = {
  valor: string;
  editavel: boolean;
  onMudar: (v: string) => void;
  rotulo: string;
};

const estiloTitulo = { fontSize: 18, fontWeight: 600 } as const;

export function InlineTitle({ valor, editavel, onMudar, rotulo }: Props): ReactNode {
  const [rascunho, setRascunho] = useState<string | null>(null);
  const cancelado = useRef(false);

  if (!editavel) {
    return (
      <Typography variant="h1" sx={estiloTitulo}>
        {valor}
      </Typography>
    );
  }

  if (rascunho === null) {
    return (
      <ButtonBase
        aria-label={`Editar ${rotulo}`}
        onClick={() => {
          cancelado.current = false;
          setRascunho(valor);
        }}
        sx={{ ...estiloTitulo, justifyContent: 'flex-start', borderRadius: 1, px: 0.5 }}
      >
        {valor}
      </ButtonBase>
    );
  }

  const confirmar = () => {
    const aparado = rascunho.trim();
    setRascunho(null);
    if (!cancelado.current && aparado !== '' && aparado !== valor) onMudar(aparado);
  };

  const aoTeclar = (e: KeyboardEvent) => {
    if (e.key === 'Enter') confirmar();
    if (e.key === 'Escape') {
      cancelado.current = true;
      setRascunho(null);
    }
  };

  return (
    <InputBase
      autoFocus
      inputProps={{ 'aria-label': rotulo }}
      value={rascunho}
      onChange={(e) => setRascunho(e.target.value)}
      onBlur={confirmar}
      onKeyDown={aoTeclar}
      sx={{ ...estiloTitulo, minWidth: 200 }}
    />
  );
}
