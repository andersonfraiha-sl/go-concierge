import { Box, Chip, Typography } from '@mui/material';
import type { KeyboardEvent, ReactNode } from 'react';
import { useForm } from 'react-hook-form';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';

type Props = {
  titulo: string;
  ajuda: string;
  rotuloNovoItem: string;
  mensagemVazia: string;
  itens: string[];
  editavel: boolean;
  onMudar: (itens: string[]) => void;
};

/** Lista de chips com adição por Enter; item em branco ou repetido nunca entra. */
export function ListaDeChips({
  titulo,
  ajuda,
  rotuloNovoItem,
  mensagemVazia,
  itens,
  editavel,
  onMudar,
}: Props): ReactNode {
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
    <SectionCard titulo={titulo} ajuda={ajuda}>
      {!editavel && itens.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          {mensagemVazia}
        </Typography>
      )}
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        {itens.map((item) => (
          <Chip
            key={item}
            label={item}
            {...(editavel && { onDelete: () => onMudar(itens.filter((i) => i !== item)) })}
          />
        ))}
      </Box>
      {editavel && (
        <CampoTexto
          control={control}
          name="rascunho"
          size="small"
          placeholder="Adicionar e apertar Enter"
          onKeyDown={aoTeclar}
          onBlur={adicionar}
          slotProps={{ htmlInput: { 'aria-label': rotuloNovoItem } }}
        />
      )}
    </SectionCard>
  );
}
