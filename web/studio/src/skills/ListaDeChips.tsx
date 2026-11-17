import { Box, Chip, Typography } from '@mui/material';
import type { KeyboardEvent, ReactNode } from 'react';
import { useForm } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';
import { PESOS, RAIO, TIPOS } from '@/theme/tokens';

type Props = {
  titulo: string;
  ajuda: string;
  rotuloNovoItem: string;
  placeholderNovoItem: string;
  mensagemVazia: string;
  /** Envolve cada chip em aspas curvas (frases ditas pelo morador). */
  comAspas?: boolean;
  itens: string[];
  editavel: boolean;
  onMudar: (itens: string[]) => void;
};

const comAspasCurvas = (texto: string): string => `“${texto}”`;

/** Lista de chips com adição por Enter; item em branco ou repetido nunca entra. */
export function ListaDeChips({
  titulo,
  ajuda,
  rotuloNovoItem,
  placeholderNovoItem,
  comAspas = false,
  ...lista
}: Props): ReactNode {
  const { mensagemVazia, itens, editavel, onMudar } = lista;
  return (
    <SectionCard titulo={titulo} ajuda={ajuda}>
      {!editavel && itens.length === 0 && (
        <Typography variant="body2" color="textSecondary">
          {mensagemVazia}
        </Typography>
      )}
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        {itens.map((item) => (
          <Chip
            key={item}
            label={comAspas ? comAspasCurvas(item) : item}
            deleteIcon={<Box component="span">×</Box>}
            sx={{
              borderRadius: RAIO.pilula,
              height: 28,
              bgcolor: 'background.neutralFill',
              fontSize: TIPOS.body2,
              fontWeight: PESOS.regular,
              color: 'text.strong',
            }}
            {...(editavel && { onDelete: () => onMudar(itens.filter((i) => i !== item)) })}
          />
        ))}
      </Box>
      {editavel && (
        <CampoNovoItem
          rotulo={rotuloNovoItem}
          placeholder={placeholderNovoItem}
          itens={itens}
          onMudar={onMudar}
        />
      )}
    </SectionCard>
  );
}

type PropsDoNovoItem = {
  rotulo: string;
  placeholder: string;
  itens: string[];
  onMudar: (itens: string[]) => void;
};

function CampoNovoItem({ rotulo, placeholder, itens, onMudar }: PropsDoNovoItem): ReactNode {
  const { control, getValues, reset } = useForm({ defaultValues: { rascunho: '' } });

  const adicionar = () => {
    const item = getValues('rascunho').trim();
    reset();
    if (item !== '' && !itens.includes(item)) onMudar([...itens, item]);
  };
  const aoTeclar = (evento: KeyboardEvent) => {
    if (evento.key !== 'Enter') return;
    evento.preventDefault();
    adicionar();
  };

  return (
    <CampoTexto
      control={control}
      name="rascunho"
      size="small"
      placeholder={placeholder}
      onKeyDown={aoTeclar}
      onBlur={adicionar}
      slotProps={{ htmlInput: { 'aria-label': rotulo } }}
    />
  );
}
