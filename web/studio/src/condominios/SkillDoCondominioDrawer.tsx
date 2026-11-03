import ExpandMoreOutlined from '@mui/icons-material/ExpandMoreOutlined';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  Drawer,
  Switch,
  ToggleButton,
  Typography,
} from '@mui/material';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { Link } from 'react-router';
import { pode, useEu, usePodeEditar } from '@/acesso/eu';
import { tituloDe } from '@/ferramentas/contratos';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import type { EstadoSalvamento } from '@/shared/autosave/useAutosave';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { iconeDaSkill } from '@/skills/icones';
import { CampoAjuste } from './CampoAjuste';
import {
  type Ajustes,
  type Caso,
  type Estado,
  ferramentasDaSkill,
  habilitacaoDe,
  PADRAO,
  salvarAjustes,
  salvarHabilitacao,
  salvarOrientacao,
  useAplicarMapeamento,
  useEscopo,
  type Valor,
} from './contratos';
import { exemploDeOrientacao, type Origem, origemDe, rotuloDoPapel } from './listaDeSkills';
import { useExecutar } from './useExecutar';

type Props = {
  estado: Estado;
  condominio: string;
  caso: Caso;
  titulo: string;
  editavel: boolean;
  onFechar: () => void;
};

const FRASE_DE_ORIGEM: Record<Origem, string> = {
  propria: 'Este condomínio tem decisão própria.',
  padrao: 'Sem decisão própria: vale o Padrão. Mudar aqui cria a decisão deste condomínio.',
  nenhuma: 'Sem decisão: ligada para todos.',
};

export function SkillDoCondominioDrawer({
  estado,
  condominio,
  caso,
  titulo,
  editavel,
  onFechar,
}: Props): ReactNode {
  const { data: eu } = useEu();
  const { executar, gravando } = useExecutar();
  const podeVerTextoDaSkill = usePodeEditar('skills.editar_textos');

  const encontrado = estado.mapeamento.condominios.find((c) => c.id === condominio);
  const nomeDoCondominio =
    condominio === PADRAO ? 'Padrão (todos)' : (encontrado?.nome ?? condominio);

  const { habilitacao, propria } = habilitacaoDe(estado.mapeamento, condominio, caso.nome);
  const travado = !editavel || gravando;

  const gravarHabilitacao = (habilitado: boolean, papeis: string[]): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao({ condominio, caso: caso.nome, habilitado, papeis, limpar: false }, escopo),
    );

  const voltarAoPadrao = (): Promise<boolean> =>
    executar((escopo) =>
      salvarHabilitacao(
        { condominio, caso: caso.nome, habilitado: true, papeis: [], limpar: true },
        escopo,
      ),
    );

  return (
    <Drawer
      anchor="right"
      open
      onClose={onFechar}
      slotProps={{
        paper: {
          sx: { width: 'min(420px, 100vw)' },
          role: 'dialog',
          'aria-label': `${titulo} · ${nomeDoCondominio}`,
        },
      }}
    >
      <Box sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 2 }}>
        <Cabecalho
          caso={caso}
          titulo={titulo}
          habilitada={habilitacao.habilitado}
          origem={origemDe(estado.mapeamento, condominio, caso.nome)}
          travado={travado}
          onMudarHabilitada={(ligado) => void gravarHabilitacao(ligado, habilitacao.papeis)}
        />
        <QuemPodeUsar
          papeis={estado.papeis}
          escolhidos={habilitacao.papeis}
          travado={travado}
          onMudar={(papeis) => void gravarHabilitacao(habilitacao.habilitado, papeis)}
        />
        <OrientacaoDaSkill
          estado={estado}
          condominio={condominio}
          caso={caso}
          editavel={editavel}
        />
        <AjustesDaSkill estado={estado} condominio={condominio} caso={caso} editavel={editavel} />
        {podeVerTextoDaSkill && <ComoReconhece descricao={caso.descricao} />}
        <Box sx={{ display: 'flex', gap: 1 }}>
          {propria && editavel && (
            <Button fullWidth onClick={() => void voltarAoPadrao()} disabled={gravando}>
              Voltar ao Padrão
            </Button>
          )}
          {eu && pode(eu, 'skills.ler') && (
            <Button
              fullWidth
              component={Link}
              to={`/skills/${encodeURIComponent(caso.nome)}?aba=fluxo`}
            >
              Ver o fluxo
            </Button>
          )}
        </Box>
      </Box>
    </Drawer>
  );
}

type CabecalhoProps = {
  caso: Caso;
  titulo: string;
  habilitada: boolean;
  origem: Origem;
  travado: boolean;
  onMudarHabilitada: (ligado: boolean) => void;
};

function Cabecalho({
  caso,
  titulo,
  habilitada,
  origem,
  travado,
  onMudarHabilitada,
}: CabecalhoProps): ReactNode {
  const Icone = iconeDaSkill(caso.icone);
  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1 }}>
        <Icone color="primary" />
        <Box sx={{ flex: 1 }}>
          <Typography variant="h6" component="h2">
            {titulo}
          </Typography>
          {caso.resumo !== '' && (
            <Typography variant="body2" color="text.secondary">
              {caso.resumo}
            </Typography>
          )}
        </Box>
        <Switch
          checked={habilitada}
          disabled={travado}
          slotProps={{ input: { 'aria-label': 'Ligada neste condomínio' } }}
          onChange={(_, ligado) => onMudarHabilitada(ligado)}
        />
      </Box>
      <Typography variant="caption" color="text.secondary">
        {FRASE_DE_ORIGEM[origem]}
      </Typography>
    </>
  );
}

type QuemPodeUsarProps = {
  papeis: readonly string[];
  escolhidos: readonly string[];
  travado: boolean;
  onMudar: (papeis: string[]) => void;
};

function QuemPodeUsar({ papeis, escolhidos, travado, onMudar }: QuemPodeUsarProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Typography variant="subtitle2" component="h3">
        Quem pode usar
      </Typography>
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        <ToggleButton
          value="todos"
          selected={escolhidos.length === 0}
          disabled={travado}
          sx={{ borderRadius: 9 }}
          onClick={() => onMudar([])}
        >
          Todos
        </ToggleButton>
        {papeis.map((papel) => {
          const marcado = escolhidos.includes(papel);
          const proximos = marcado ? escolhidos.filter((p) => p !== papel) : [...escolhidos, papel];
          return (
            <ToggleButton
              key={papel}
              value={papel}
              selected={marcado}
              disabled={travado}
              sx={{ borderRadius: 9 }}
              onClick={() => onMudar(proximos)}
            >
              {rotuloDoPapel(papel)}
            </ToggleButton>
          );
        })}
      </Box>
    </Box>
  );
}

type SecaoProps = { estado: Estado; condominio: string; caso: Caso; editavel: boolean };

function OrientacaoDaSkill({ estado, condominio, caso, editavel }: SecaoProps): ReactNode {
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
      titulo="Orientação para esta skill"
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

function AjustesDaSkill({ estado, condominio, caso, editavel }: SecaoProps): ReactNode {
  const escopo = useEscopo() ?? null;
  const aplicar = useAplicarMapeamento();
  const ferramentas = ferramentasDaSkill(estado, caso);

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

type SecaoComSalvamentoProps = {
  titulo: string;
  salvamento: EstadoSalvamento;
  tentarDeNovo: () => Promise<void>;
  children: ReactNode;
};

function SecaoComSalvamento({
  titulo,
  salvamento,
  tentarDeNovo,
  children,
}: SecaoComSalvamentoProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="subtitle2" component="h3" sx={{ flex: 1 }}>
          {titulo}
        </Typography>
        <SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />
      </Box>
      {children}
    </Box>
  );
}

function ComoReconhece({ descricao }: { descricao: string }): ReactNode {
  return (
    <Accordion disableGutters sx={{ boxShadow: 'none', border: 1, borderColor: 'divider' }}>
      <AccordionSummary expandIcon={<ExpandMoreOutlined />}>
        <Typography variant="subtitle2" component="h3">
          Como o concierge reconhece o pedido
        </Typography>
      </AccordionSummary>
      <AccordionDetails>
        <Typography variant="body2" color="text.secondary">
          {descricao}
        </Typography>
      </AccordionDetails>
    </Accordion>
  );
}
