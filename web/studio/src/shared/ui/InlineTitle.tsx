import { ButtonBase, InputBase, Typography } from '@mui/material';
import { type KeyboardEvent, type ReactNode, useRef, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';

type Props = {
  valor: string;
  editavel: boolean;
  onMudar: (v: string) => void;
  rotulo: string;
  /** Tag renderizada quando não editável; "h1" é o título da página (padrão), "span" para usos em lista. */
  component?: 'h1' | 'span';
};

const estiloTitulo = { fontSize: 18, fontWeight: 600 } as const;

export function InlineTitle({
  valor,
  editavel,
  onMudar,
  rotulo,
  component = 'h1',
}: Props): ReactNode {
  const [editando, setEditando] = useState(false);

  if (!editavel) {
    return (
      <Typography variant="h1" component={component} sx={estiloTitulo}>
        {valor}
      </Typography>
    );
  }
  if (!editando) {
    return (
      <ButtonBase
        aria-label={`Editar ${rotulo}`}
        onClick={() => setEditando(true)}
        sx={{ ...estiloTitulo, justifyContent: 'flex-start', borderRadius: 1, px: 0.5 }}
      >
        {valor}
      </ButtonBase>
    );
  }
  return (
    <CampoDoTitulo
      valor={valor}
      rotulo={rotulo}
      onMudar={onMudar}
      onEncerrar={() => setEditando(false)}
    />
  );
}

type PropsDoCampo = {
  valor: string;
  rotulo: string;
  onMudar: (v: string) => void;
  onEncerrar: () => void;
};

/** Monte só enquanto edita: o rascunho nasce do valor atual. */
function CampoDoTitulo({ valor, rotulo, onMudar, onEncerrar }: PropsDoCampo): ReactNode {
  const { control, getValues } = useForm({ defaultValues: { rascunho: valor } });
  const cancelado = useRef(false);

  const confirmar = () => {
    const aparado = getValues('rascunho').trim();
    onEncerrar();
    if (!cancelado.current && aparado !== '' && aparado !== valor) onMudar(aparado);
  };

  const aoTeclar = (e: KeyboardEvent) => {
    if (e.key === 'Enter') confirmar();
    if (e.key === 'Escape') {
      cancelado.current = true;
      onEncerrar();
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
