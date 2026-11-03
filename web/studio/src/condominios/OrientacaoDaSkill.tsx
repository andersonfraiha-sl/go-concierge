import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import {
  type Caso,
  type Estado,
  salvarOrientacao,
  useAplicarMapeamento,
  useEscopo,
} from './contratos';
import { exemploDeOrientacao } from './listaDeSkills';
import { SecaoComSalvamento } from './SecaoComSalvamento';

type Props = { estado: Estado; condominio: string; caso: Caso; editavel: boolean };

export function OrientacaoDaSkill({ estado, condominio, caso, editavel }: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const { control } = useForm({
    defaultValues: { texto: estado.mapeamento.orientacoes[condominio]?.por_caso[caso.nome] ?? '' },
  });
  const texto = useWatch({ control, name: 'texto' });

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: texto,
    salvar: async (valor) => {
      const atual = estado.mapeamento.orientacoes[condominio] ?? { geral: '', por_caso: {} };
      const porCaso = Object.fromEntries(
        Object.entries({ ...atual.por_caso, [caso.nome]: valor }).filter(
          ([, texto]) => texto.trim() !== '',
        ),
      );
      aplicar(
        await salvarOrientacao({ condominio, geral: atual.geral, por_caso: porCaso }, escopo),
      );
    },
  });

  return (
    <SecaoComSalvamento
      titulo="Orientação desta skill"
      salvamento={salvamento}
      tentarDeNovo={tentarDeNovo}
    >
      <CampoTexto
        control={control}
        name="texto"
        multiline
        minRows={3}
        fullWidth
        disabled={!editavel}
        placeholder={`Ex.: ${exemploDeOrientacao(caso.nome)}`}
      />
    </SecaoComSalvamento>
  );
}
