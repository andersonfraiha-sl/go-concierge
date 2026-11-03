import AddIcon from '@mui/icons-material/Add';
import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { Box, Button, Chip, IconButton, TextField, Typography } from '@mui/material';
import { useState, type KeyboardEvent, type ReactNode } from 'react';
import { SectionCard } from '@/shared/ui/SectionCard';
import type { Textos } from './contratos';

type Props = {
  textos: Textos;
  editavel: boolean;
  onMudar: (textos: Textos) => void;
};

export function QuandoUsarAba({ textos, editavel, onMudar }: Props): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <SectionCard titulo="Quando usar" ajuda="Diga em uma frase quando a assistente deve usar esta skill.">
        <TextField
          multiline
          minRows={2}
          fullWidth
          disabled={!editavel}
          value={textos.quandoUsar}
          onChange={(e) => onMudar({ ...textos, quandoUsar: e.target.value })}
          slotProps={{ htmlInput: { 'aria-label': 'Quando usar' } }}
        />
      </SectionCard>
      {textos.temFluxo && (
        <>
          <PalavrasChave
            palavras={textos.palavrasChave}
            editavel={editavel}
            onMudar={(palavrasChave) => onMudar({ ...textos, palavrasChave })}
          />
          <FrasesDeExemplo
            frases={textos.frasesDeExemplo}
            editavel={editavel}
            onMudar={(frasesDeExemplo) => onMudar({ ...textos, frasesDeExemplo })}
          />
        </>
      )}
    </Box>
  );
}

type ListaProps = { editavel: boolean };

function PalavrasChave({
  palavras,
  editavel,
  onMudar,
}: ListaProps & { palavras: string[]; onMudar: (palavras: string[]) => void }): ReactNode {
  const [rascunho, setRascunho] = useState('');

  const adicionar = () => {
    const palavra = rascunho.trim();
    setRascunho('');
    if (palavra !== '' && !palavras.includes(palavra)) onMudar([...palavras, palavra]);
  };
  const aoTeclar = (e: KeyboardEvent) => {
    if (e.key !== 'Enter') return;
    e.preventDefault();
    adicionar();
  };

  return (
    <SectionCard titulo="Palavras-chave" ajuda="Palavras que ajudam a reconhecer o pedido.">
      {!editavel && palavras.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          Nenhuma palavra-chave.
        </Typography>
      )}
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        {palavras.map((palavra) => (
          <Chip
            key={palavra}
            label={palavra}
            {...(editavel && { onDelete: () => onMudar(palavras.filter((p) => p !== palavra)) })}
          />
        ))}
      </Box>
      {editavel && (
        <TextField
          size="small"
          placeholder="Adicionar palavra e apertar Enter"
          value={rascunho}
          onChange={(e) => setRascunho(e.target.value)}
          onKeyDown={aoTeclar}
          onBlur={adicionar}
          slotProps={{ htmlInput: { 'aria-label': 'Nova palavra-chave' } }}
        />
      )}
    </SectionCard>
  );
}

function FrasesDeExemplo({
  frases,
  editavel,
  onMudar,
}: ListaProps & { frases: string[]; onMudar: (frases: string[]) => void }): ReactNode {
  const trocar = (posicao: number, texto: string) => onMudar(frases.map((f, i) => (i === posicao ? texto : f)));
  const remover = (posicao: number) => onMudar(frases.filter((_, i) => i !== posicao));

  return (
    <SectionCard titulo="Frases de exemplo" ajuda="Frases que um morador diria para pedir isto.">
      {frases.map((frase, posicao) => (
        // A posição é a identidade: a frase muda a cada tecla, então o texto não serve de chave.
        <Box key={posicao} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <TextField
            size="small"
            fullWidth
            disabled={!editavel}
            value={frase}
            onChange={(e) => trocar(posicao, e.target.value)}
            slotProps={{ htmlInput: { 'aria-label': `Frase de exemplo ${posicao + 1}` } }}
          />
          {editavel && (
            <IconButton aria-label={`Remover frase ${posicao + 1}`} onClick={() => remover(posicao)}>
              <DeleteOutlinedIcon fontSize="small" />
            </IconButton>
          )}
        </Box>
      ))}
      {editavel && (
        <Button size="small" startIcon={<AddIcon />} sx={{ alignSelf: 'flex-start' }} onClick={() => onMudar([...frases, ''])}>
          Adicionar frase
        </Button>
      )}
    </SectionCard>
  );
}
