import { Box, Button, TextField, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { SectionCard } from '@/shared/ui/SectionCard';
import { CampoAjuste } from './CampoAjuste';
import {
  type Ajustes,
  type Estado,
  PADRAO,
  salvarAjustes,
  sugerirAjustes,
  useAplicarMapeamento,
  useEscopo,
  type Valor,
} from './contratos';

type Props = { estado: Estado; condominio: string; editavel: boolean };

export function AjustesSecao({ estado, condominio, editavel }: Props): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const [ajustes, setAjustes] = useState<Ajustes>(estado.mapeamento.payloads[condominio] ?? {});
  const ferramentas = estado.recursos.filter((r) => r.definicoes.length > 0);

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: ajustes,
    salvar: async (valor) => {
      aplicar(await salvarAjustes({ condominio, ajustes: valor }, escopo));
    },
  });

  const mudar = (recurso: string, campos: Record<string, Valor>): void =>
    setAjustes((atual) => ({ ...atual, [recurso]: { ...atual[recurso], ...campos } }));

  const padraoDe = (recurso: string): Record<string, Valor> =>
    condominio === PADRAO ? {} : (estado.mapeamento.payloads[PADRAO]?.[recurso] ?? {});

  return (
    <SectionCard
      titulo="Ajustes"
      ajuda="Prazos, horários e códigos deste condomínio."
      acao={<SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />}
    >
      {ferramentas.length === 0 && (
        <Typography color="text.secondary">
          Nenhuma ferramenta tem ajustes por condomínio.
        </Typography>
      )}
      {ferramentas.map((ferramenta) => (
        <AjustesDaFerramenta
          key={ferramenta.name}
          ferramenta={ferramenta}
          condominio={condominio}
          valores={ajustes[ferramenta.name] ?? {}}
          padrao={padraoDe(ferramenta.name)}
          editavel={editavel}
          onMudar={(campos) => mudar(ferramenta.name, campos)}
        />
      ))}
    </SectionCard>
  );
}

type FerramentaProps = {
  ferramenta: Ferramenta;
  condominio: string;
  valores: Record<string, Valor>;
  padrao: Record<string, Valor>;
  editavel: boolean;
  onMudar: (campos: Record<string, Valor>) => void;
};

function AjustesDaFerramenta({
  ferramenta,
  condominio,
  valores,
  padrao,
  editavel,
  onMudar,
}: FerramentaProps): ReactNode {
  const escopo = useEscopo() ?? null;
  const [frase, setFrase] = useState('');
  const [pensando, setPensando] = useState(false);
  const [aviso, setAviso] = useState<string | null>(null);

  async function sugerir(): Promise<void> {
    setPensando(true);
    setAviso(null);
    try {
      const sugestao = await sugerirAjustes(
        { condominio, recurso: ferramenta.name, texto: frase, atual: valores },
        escopo,
      );
      onMudar(sugestao.payload);
      const semNada = Object.keys(sugestao.payload).length === 0;
      setAviso(
        sugestao.nota ||
          (semNada ? 'Não encontrei nada no texto.' : 'Revise os campos preenchidos.'),
      );
    } catch (erro) {
      setAviso(erro instanceof Error ? erro.message : String(erro));
    } finally {
      setPensando(false);
    }
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <Typography variant="subtitle2">{ferramenta.titulo ?? ferramenta.name}</Typography>
      {editavel && (
        <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
          <TextField
            size="small"
            fullWidth
            label="Descreva em uma frase"
            value={frase}
            onChange={(e) => setFrase(e.target.value)}
          />
          <Button
            variant="outlined"
            disabled={pensando || frase.trim() === ''}
            onClick={() => void sugerir()}
          >
            {pensando ? 'Pensando…' : 'Sugerir'}
          </Button>
        </Box>
      )}
      {aviso !== null && (
        <Typography variant="caption" color="text.secondary" role="status">
          {aviso}
        </Typography>
      )}
      {ferramenta.definicoes.map((definicao) => (
        <CampoAjuste
          key={definicao.name}
          definicao={definicao}
          valor={valores[definicao.name]}
          herdado={padrao[definicao.name] ?? definicao.default}
          desabilitado={!editavel}
          onMudar={(valor) => onMudar({ [definicao.name]: valor })}
        />
      ))}
    </Box>
  );
}
