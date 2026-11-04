import type { ReactNode } from 'react';
import { type Control, useForm, useWatch } from 'react-hook-form';
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
type Campos = { texto: string };

function porCasoComTexto(porCaso: Record<string, string>, caso: string, texto: string) {
  const comNovo = { ...porCaso, [caso]: texto };
  return Object.fromEntries(
    Object.entries(comNovo).filter(([, preenchido]) => preenchido.trim() !== ''),
  );
}

function useOrientacaoDaSkill({ estado, condominio, caso }: Omit<Props, 'editavel'>) {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const { control } = useForm<Campos>({
    defaultValues: { texto: estado.mapeamento.orientacoes[condominio]?.por_caso[caso.nome] ?? '' },
  });
  const texto = useWatch({ control, name: 'texto' });

  const autosave = useAutosave({
    valor: texto,
    salvar: async (valor) => {
      const atual = estado.mapeamento.orientacoes[condominio] ?? { geral: '', por_caso: {} };
      const porCaso = porCasoComTexto(atual.por_caso, caso.nome, valor);
      aplicar(
        await salvarOrientacao({ condominio, geral: atual.geral, por_caso: porCaso }, escopo),
      );
    },
  });

  return { control, salvamento: autosave.estado, tentarDeNovo: autosave.tentarDeNovo };
}

export function OrientacaoDaSkill(props: Props): ReactNode {
  const { control, salvamento, tentarDeNovo } = useOrientacaoDaSkill(props);

  return (
    <SecaoComSalvamento
      titulo="Orientação desta skill"
      salvamento={salvamento}
      tentarDeNovo={tentarDeNovo}
    >
      <CampoDaOrientacao control={control} caso={props.caso} editavel={props.editavel} />
    </SecaoComSalvamento>
  );
}

type CampoProps = { control: Control<Campos>; caso: Caso; editavel: boolean };

function CampoDaOrientacao({ control, caso, editavel }: CampoProps): ReactNode {
  return (
    <CampoTexto
      control={control}
      name="texto"
      multiline
      minRows={3}
      fullWidth
      disabled={!editavel}
      placeholder={`Ex.: ${exemploDeOrientacao(caso.nome)}`}
    />
  );
}
