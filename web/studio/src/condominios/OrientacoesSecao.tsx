import { Box, TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { useValoresDoFormulario } from '@/shared/autosave/useValoresDoFormulario';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  type Estado,
  type Orientacao,
  salvarOrientacao,
  useAplicarMapeamento,
  useEscopo,
} from './contratos';

type Props = {
  estado: Estado;
  condominio: string;
  titulos: ReadonlyMap<string, string>;
  editavel: boolean;
};

const SEM_ORIENTACAO: Orientacao = { geral: '', por_caso: {} };

const semTextosVazios = (porCaso: Record<string, string>): Record<string, string> =>
  Object.fromEntries(Object.entries(porCaso).filter(([, texto]) => texto.trim() !== ''));

export function OrientacoesSecao({ estado, condominio, titulos, editavel }: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const { control } = useForm<Orientacao>({
    defaultValues: estado.mapeamento.orientacoes[condominio] ?? SEM_ORIENTACAO,
  });

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: useValoresDoFormulario(control),
    salvar: async ({ geral, por_caso }) => {
      aplicar(
        await salvarOrientacao({ condominio, geral, por_caso: semTextosVazios(por_caso) }, escopo),
      );
    },
  });

  return (
    <SectionCard
      titulo="Orientações"
      ajuda="O que a assistente precisa saber deste condomínio."
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
        <CampoTexto
          control={control}
          name="geral"
          label="Para todas as skills"
          multiline
          minRows={3}
          disabled={!editavel}
        />
        {/* Um campo só para o mapa: nome de skill com ponto viraria caminho aninhado no formulário. */}
        <Controller
          control={control}
          name="por_caso"
          render={({ field }) => (
            <>
              {estado.casos.map((caso) => (
                <TextField
                  key={caso.nome}
                  label={titulos.get(caso.nome) ?? caso.titulo}
                  multiline
                  minRows={2}
                  disabled={!editavel}
                  value={field.value[caso.nome] ?? ''}
                  onChange={(e) => field.onChange({ ...field.value, [caso.nome]: e.target.value })}
                />
              ))}
            </>
          )}
        />
      </Box>
    </SectionCard>
  );
}
