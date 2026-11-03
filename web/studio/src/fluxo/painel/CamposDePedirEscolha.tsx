import { FormControlLabel, Switch } from '@mui/material';
import type { ReactNode } from 'react';
import type { CamposProps } from './camposPorTipo';
import { Texto } from './Texto';

export function CamposDePedirEscolha({
  bloco,
  editavel,
  onMudar,
}: CamposProps<'pedir_escolha'>): ReactNode {
  const texto = (
    rotulo: string,
    chave: 'param' | 'titulo' | 'de' | 'rotulo' | 'valor' | 'pergunta' | 'seVazio',
  ) => (
    <Texto
      rotulo={rotulo}
      valor={bloco[chave]}
      editavel={editavel}
      onMudar={(valor) => onMudar({ ...bloco, [chave]: valor })}
    />
  );
  return (
    <>
      {texto('Guardar escolha em', 'param')}
      {texto('Título', 'titulo')}
      {texto('Opções vêm de', 'de')}
      {texto('Mostrar o campo', 'rotulo')}
      {texto('Usar o valor do campo', 'valor')}
      {texto('Pergunta', 'pergunta')}
      {texto('Se não houver opções', 'seVazio')}
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
