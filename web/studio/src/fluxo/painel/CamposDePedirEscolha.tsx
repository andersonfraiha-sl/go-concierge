import { FormControlLabel, Switch } from '@mui/material';
import type { ReactNode } from 'react';
import type { CamposProps } from './camposPorTipo';
import { Texto } from './Texto';

type Chave = 'param' | 'titulo' | 'de' | 'rotulo' | 'valor' | 'pergunta' | 'seVazio';

/** Rótulo e campo do bloco, na ordem em que aparecem na tela. */
const CAMPOS_DE_TEXTO: readonly (readonly [string, Chave])[] = [
  ['Guardar escolha em', 'param'],
  ['Título', 'titulo'],
  ['Opções vêm de', 'de'],
  ['Mostrar o campo', 'rotulo'],
  ['Usar o valor do campo', 'valor'],
  ['Pergunta', 'pergunta'],
  ['Se não houver opções', 'seVazio'],
];

export function CamposDePedirEscolha({
  bloco,
  editavel,
  onMudar,
}: CamposProps<'pedir_escolha'>): ReactNode {
  return (
    <>
      {CAMPOS_DE_TEXTO.map(([rotulo, chave]) => (
        <Texto
          key={chave}
          rotulo={rotulo}
          valor={bloco[chave]}
          editavel={editavel}
          onMudar={(valor) => onMudar({ ...bloco, [chave]: valor })}
        />
      ))}
      <FormControlLabel
        label="Vale para a conversa toda"
        disabled={!editavel}
        control={
          <Switch
            checked={bloco.reaproveitar === 'always'}
            onChange={(_, ligado) => onMudar({ ...bloco, reaproveitar: ligado ? 'always' : '' })}
          />
        }
      />
    </>
  );
}
