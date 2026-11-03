import { Box, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { SectionCard } from '@/shared/ui/SectionCard';
import { CampoAjuste } from './CampoAjuste';
import {
  type Ajustes,
  type Estado,
  ferramentasGerais,
  PADRAO,
  salvarAjustes,
  useAplicarMapeamento,
  useEscopo,
  type Valor,
} from './contratos';
import { PreencherComIA } from './PreencherComIA';

type Props = { estado: Estado; condominio: string; editavel: boolean };

export function AjustesSecao({ estado, condominio, editavel }: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const ferramentas = ferramentasGerais(estado);
  const payloadsDoCondominio = estado.mapeamento.payloads[condominio] ?? {};
  // Fora do react-hook-form de propósito: Valor é JSON recursivo e os tipos de caminho do RHF
  // não instanciam sobre ele (TS2589). Cada CampoAjuste já é um campo controlado deste mapa.
  const [ajustes, setAjustes] = useState<Ajustes>(
    Object.fromEntries(ferramentas.map((f) => [f.name, payloadsDoCondominio[f.name] ?? {}])),
  );

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: ajustes,
    salvar: async (valor) => {
      // Mescla com o que está no cache agora: o drawer de cada skill grava a própria parte deste
      // mesmo mapa, e esta seção não pode apagá-la.
      const atuais = estado.mapeamento.payloads[condominio] ?? {};
      aplicar(await salvarAjustes({ condominio, ajustes: { ...atuais, ...valor } }, escopo));
    },
  });

  const mudar = (recurso: string, campos: Record<string, Valor>): void =>
    setAjustes((atual) => ({ ...atual, [recurso]: { ...atual[recurso], ...campos } }));

  const herdadoDe = (recurso: string): Record<string, Valor> =>
    condominio === PADRAO ? {} : (estado.mapeamento.payloads[PADRAO]?.[recurso] ?? {});

  return (
    <SectionCard
      titulo="Ajustes gerais"
      ajuda="Prazos, horários e códigos que não são de uma skill só. Campo vazio vale o padrão."
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
      {ferramentas.length === 0 && (
        <Typography color="text.secondary">
          Os ajustes de cada skill ficam na própria skill, na lista acima.
        </Typography>
      )}
      {ferramentas.length > 0 && editavel && (
        <PreencherComIA
          condominio={condominio}
          ferramentas={ferramentas}
          valores={ajustes}
          onPreencher={mudar}
        />
      )}
      {ferramentas.map((ferramenta) => (
        <Box key={ferramenta.name} sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
          <Typography variant="subtitle2">{ferramenta.titulo ?? ferramenta.name}</Typography>
          {ferramenta.definicoes.map((definicao) => (
            <CampoAjuste
              key={definicao.name}
              definicao={definicao}
              valor={ajustes[ferramenta.name]?.[definicao.name]}
              herdado={herdadoDe(ferramenta.name)[definicao.name] ?? definicao.default}
              desabilitado={!editavel}
              onMudar={(valor) => mudar(ferramenta.name, { [definicao.name]: valor })}
            />
          ))}
        </Box>
      ))}
    </SectionCard>
  );
}
