import { ButtonBase, InputBase, Typography } from '@mui/material';
import { type KeyboardEvent, type ReactNode, useRef, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { PESOS, RAIO, TIPOS } from '@/theme/tokens';

type Props = {
  valor: string;
  editavel: boolean;
  onMudar: (v: string) => void;
  rotulo: string;
  /** Tag renderizada quando não editável; "h1" é o título da página (padrão), "span" para usos em lista. */
  component?: 'h1' | 'span';
  /** "herdada" usa a fonte de quem envolve (ex.: o título do cabeçalho de detalhe). */
  aparencia?: Aparencia;
};

type Aparencia = 'titulo' | 'herdada';

const ESTILO_POR_APARENCIA = {
  titulo: { fontSize: TIPOS.tituloEditavel, fontWeight: PESOS.semi },
  herdada: { font: 'inherit', color: 'inherit', letterSpacing: 'inherit' },
} as const satisfies Record<Aparencia, object>;

export function InlineTitle({
  valor,
  editavel,
  onMudar,
  rotulo,
  component = 'h1',
  aparencia = 'titulo',
}: Props): ReactNode {
  const [editando, setEditando] = useState(false);
  const estiloTitulo = ESTILO_POR_APARENCIA[aparencia];

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
        sx={{
          ...estiloTitulo,
          justifyContent: 'flex-start',
          borderRadius: RAIO.campo,
          px: aparencia === 'titulo' ? 0.5 : 0,
        }}
      >
        {valor}
      </ButtonBase>
    );
  }
  return (
    <CampoDoTitulo
      estilo={estiloTitulo}
      valor={valor}
      rotulo={rotulo}
      onMudar={onMudar}
      onEncerrar={() => setEditando(false)}
    />
  );
}

type PropsDoCampo = {
  estilo: (typeof ESTILO_POR_APARENCIA)[Aparencia];
  valor: string;
  rotulo: string;
  onMudar: (v: string) => void;
  onEncerrar: () => void;
};

/** Monte só enquanto edita: o rascunho nasce do valor atual. */
function CampoDoTitulo({ estilo, valor, rotulo, onMudar, onEncerrar }: PropsDoCampo): ReactNode {
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
          sx={{ ...estilo, minWidth: 200 }}
        />
      )}
    />
  );
}
