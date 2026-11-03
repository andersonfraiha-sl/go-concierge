import { ButtonBase, InputBase, Typography } from '@mui/material';
import { type KeyboardEvent, type ReactNode, useRef, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

type Props = {
  valor: string;
  editavel: boolean;
  onMudar: (v: string) => void;
  rotulo: string;
};

const estiloTitulo = { fontSize: 18, fontWeight: 600 } as const;

export function InlineTitle({ valor, editavel, onMudar, rotulo }: Props): ReactNode {
  const [editando, setEditando] = useState(false);
  const { control, getValues, reset } = useForm({ defaultValues: { rascunho: valor } });
  const cancelado = useRef(false);

  if (!editavel) {
    return (
      <Typography variant="h1" sx={estiloTitulo}>
        {valor}
      </Typography>
    );
  }

  if (!editando) {
    return (
      <ButtonBase
        aria-label={`Editar ${rotulo}`}
        onClick={() => {
          cancelado.current = false;
          reset({ rascunho: valor });
          setEditando(true);
        }}
        sx={{ ...estiloTitulo, justifyContent: 'flex-start', borderRadius: 1, px: 0.5 }}
      >
        {valor}
      </ButtonBase>
    );
  }

  const confirmar = () => {
    const aparado = getValues('rascunho').trim();
    setEditando(false);
    if (!cancelado.current && aparado !== '' && aparado !== valor) onMudar(aparado);
  };

  const aoTeclar = (e: KeyboardEvent) => {
    if (e.key === 'Enter') confirmar();
    if (e.key === 'Escape') {
      cancelado.current = true;
      setEditando(false);
    }
  };

  return (
    <Controller
      control={control}
      name="rascunho"
      render={({ field }) => (
        <InputBase
          autoFocus
          inputProps={{ 'aria-label': rotulo }}
          value={field.value}
          onChange={field.onChange}
          onBlur={confirmar}
          onKeyDown={aoTeclar}
          sx={{ ...estiloTitulo, minWidth: 200 }}
        />
      )}
    />
  );
}
