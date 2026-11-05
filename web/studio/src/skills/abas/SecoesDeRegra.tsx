import DeleteOutlinedIcon from '@mui/icons-material/DeleteOutlined';
import { Box, IconButton, Paper, Typography } from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';
import { useFieldArray, useForm, useWatch } from 'react-hook-form';
import { chaveDoArquivo, salvarArquivo } from '@/catalogo/contratos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { CampoRico } from '@/shared/editor/CampoRico';
import { juntarSecoes, type SecaoDeMarkdown } from './secoesDoMarkdown';

type FormularioDeSecoes = { secoes: SecaoDeMarkdown[] };

type Props = {
  agente: string;
  caminho: string;
  secoes: readonly SecaoDeMarkdown[];
  editavel: boolean;
  /** Apaga o arquivo inteiro: usado quando a seção removida é a última que resta. */
  onExcluirArquivo?: (() => void) | undefined;
  descartar?: boolean | undefined;
};

/** Um arquivo de regra com título(s) `## `: cada um vira o seu próprio cartão. */
export function SecoesDeRegra({
  agente,
  caminho,
  secoes,
  editavel,
  onExcluirArquivo,
  descartar = false,
}: Props): ReactNode {
  const cliente = useQueryClient();
  const { control } = useForm<FormularioDeSecoes>({ defaultValues: { secoes: [...secoes] } });
  const { fields, remove } = useFieldArray({ control, name: 'secoes' });
  const valor = useWatch({ control, name: 'secoes' });

  const salvar = async (atuais: SecaoDeMarkdown[]): Promise<void> => {
    const conteudo = juntarSecoes(atuais);
    await salvarArquivo({ agente, caminho, conteudo });
    cliente.setQueryData(chaveDoArquivo(agente, caminho), { caminho, conteudo });
  };
  const { estado, tentarDeNovo } = useAutosave({
    valor,
    salvar,
    bloqueio: descartar ? 'arquivo em exclusão' : null,
  });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      {fields.map((campo, indice) => (
        <CartaoDaSecao
          key={campo.id}
          titulo={valor[indice]?.titulo ?? campo.titulo}
          control={control}
          indice={indice}
          editavel={editavel}
          estado={indice === 0 ? estado : undefined}
          onTentarDeNovo={() => void tentarDeNovo()}
          // Com uma seção só, excluí-la excluiria o arquivo: delega pro fluxo de exclusão
          // com confirmação. Com várias, cada cartão some por conta própria.
          onExcluir={fields.length > 1 ? () => remove(indice) : onExcluirArquivo}
        />
      ))}
    </Box>
  );
}

type PropsDoCartao = {
  titulo: string;
  control: ReturnType<typeof useForm<FormularioDeSecoes>>['control'];
  indice: number;
  editavel: boolean;
  estado: ReturnType<typeof useAutosave>['estado'] | undefined;
  onTentarDeNovo: () => void;
  onExcluir: (() => void) | undefined;
};

function CartaoDaSecao({
  titulo,
  control,
  indice,
  editavel,
  estado,
  onTentarDeNovo,
  onExcluir,
}: PropsDoCartao): ReactNode {
  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="subtitle2" sx={{ flex: 1 }}>
          {titulo}
        </Typography>
        {estado && <SaveIndicator estado={estado} onTentarDeNovo={onTentarDeNovo} />}
        {editavel && onExcluir && (
          <IconButton size="small" aria-label={`Excluir ${titulo}`} onClick={onExcluir}>
            <DeleteOutlinedIcon fontSize="small" />
          </IconButton>
        )}
      </Box>
      <CampoRico
        control={control}
        name={`secoes.${indice}.corpo`}
        rotulo={titulo}
        editavel={editavel}
      />
    </Paper>
  );
}
