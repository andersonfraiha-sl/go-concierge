import { Box, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { tituloDe } from '@/ferramentas/contratos';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { CampoAjuste } from './CampoAjuste';
import {
  type Ajustes,
  type Caso,
  type Estado,
  ferramentasDaSkill,
  PADRAO,
  salvarAjustes,
  useAplicarMapeamento,
  useEscopo,
  type Valor,
} from './contratos';
import { SecaoComSalvamento } from './SecaoComSalvamento';

type Props = { estado: Estado; condominio: string; caso: Caso; editavel: boolean };

export function AjustesDaSkill({ estado, condominio, caso, editavel }: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const ferramentas = ferramentasDaSkill(estado, caso);

  // Fora do react-hook-form: Valor é JSON recursivo e os tipos de caminho do RHF não
  // instanciam sobre ele (TS2589), como em AjustesSecao.tsx.
  const [ajustes, setAjustes] = useState<Ajustes>(() => {
    const atual = estado.mapeamento.payloads[condominio] ?? {};
    return Object.fromEntries(ferramentas.map((f) => [f.name, atual[f.name] ?? {}]));
  });

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: ajustes,
    salvar: async (valor) => {
      const atual = estado.mapeamento.payloads[condominio] ?? {};
      aplicar(await salvarAjustes({ condominio, ajustes: { ...atual, ...valor } }, escopo));
    },
  });

  const mudar = (recurso: string, campos: Record<string, Valor>): void =>
    setAjustes((atual) => ({ ...atual, [recurso]: { ...atual[recurso], ...campos } }));

  const padraoDe = (recurso: string): Record<string, Valor> =>
    condominio === PADRAO ? {} : (estado.mapeamento.payloads[PADRAO]?.[recurso] ?? {});

  if (ferramentas.length === 0) return null;

  return (
    <SecaoComSalvamento
      titulo="Ajustes desta skill"
      salvamento={salvamento}
      tentarDeNovo={tentarDeNovo}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {ferramentas.map((ferramenta) => {
          const padrao = padraoDe(ferramenta.name);
          return (
            <Box key={ferramenta.name} sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <Typography variant="caption" color="text.secondary">
                {tituloDe(ferramenta)}
              </Typography>
              {ferramenta.definicoes.map((definicao) => (
                <CampoAjuste
                  key={definicao.name}
                  definicao={definicao}
                  valor={ajustes[ferramenta.name]?.[definicao.name]}
                  herdado={padrao[definicao.name] ?? definicao.default}
                  desabilitado={!editavel}
                  onMudar={(valor) => mudar(ferramenta.name, { [definicao.name]: valor })}
                />
              ))}
            </Box>
          );
        })}
      </Box>
    </SecaoComSalvamento>
  );
}
