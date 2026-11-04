import { Box, Button, TextField, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { CaixaRica } from './CaixaRica';
import { type Aberto, abrir } from './markdownFiel';

export type PropsDoEditorRico = {
  /** Markdown, como o arquivo guarda. */
  valor: string;
  onChange: (markdown: string) => void;
  rotulo: string;
  editavel: boolean;
  minLinhas?: number;
  placeholder?: string;
  realce?: { transition?: string; bgcolor?: string };
};

type Modo = { tipo: 'rico'; aberto: Aberto } | { tipo: 'texto' };

const modoPara = (markdown: string): Modo => {
  const aberto = abrir(markdown);
  return aberto === null ? { tipo: 'texto' } : { tipo: 'rico', aberto };
};

/**
 * Campo de texto longo com formatação. Guarda Markdown: o que não foi editado volta byte a byte.
 * Texto com sintaxe que o editor não representa (tabela, HTML) abre como texto puro.
 */
export function EditorRico({
  valor,
  onChange,
  minLinhas = 4,
  ...props
}: PropsDoEditorRico): ReactNode {
  const [modo, setModo] = useState<Modo>(() => modoPara(valor));
  // O último Markdown visto: valor diferente sem ter saído daqui veio de fora (servidor, modelo, tom).
  const [visto, setVisto] = useState(valor);
  if (valor !== visto) {
    setVisto(valor);
    setModo((atual) => (atual.tipo === 'texto' ? atual : modoPara(valor)));
  }
  const emitir = (markdown: string): void => {
    setVisto(markdown);
    onChange(markdown);
  };

  const cabeNoEditor = modo.tipo === 'rico' || abrir(valor) !== null;
  const alternar = (): void => setModo(modo.tipo === 'rico' ? { tipo: 'texto' } : modoPara(valor));

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
      {modo.tipo === 'rico' ? (
        <CaixaRica aberto={modo.aberto} onChange={emitir} minLinhas={minLinhas} {...props} />
      ) : (
        <TextField
          value={valor}
          onChange={(evento) => emitir(evento.target.value)}
          multiline
          minRows={minLinhas}
          fullWidth
          placeholder={props.placeholder}
          sx={{ '& .MuiInputBase-root': props.realce ?? {} }}
          slotProps={{ htmlInput: { readOnly: !props.editavel, 'aria-label': props.rotulo } }}
        />
      )}
      {props.editavel && (
        <RodapeDoModo modo={modo.tipo} cabeNoEditor={cabeNoEditor} onAlternar={alternar} />
      )}
    </Box>
  );
}

type PropsDoRodape = { modo: Modo['tipo']; cabeNoEditor: boolean; onAlternar: () => void };

function RodapeDoModo({ modo, cabeNoEditor, onAlternar }: PropsDoRodape): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: 1 }}>
      {!cabeNoEditor && (
        <Typography variant="caption" color="text.secondary" sx={{ flex: 1 }}>
          Este texto tem formatação que o editor não mostra (tabela, HTML ou cabeçalho de
          metadados); edite como texto para não perder nada.
        </Typography>
      )}
      {cabeNoEditor && (
        <Button
          size="small"
          onClick={onAlternar}
          sx={{ minWidth: 0, py: 0, typography: 'caption' }}
        >
          {modo === 'rico' ? 'Editar como texto' : 'Voltar ao editor'}
        </Button>
      )}
    </Box>
  );
}
