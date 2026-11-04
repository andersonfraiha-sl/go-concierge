import SearchOutlined from '@mui/icons-material/SearchOutlined';
import { Box, InputAdornment, Switch, TextField, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { RAIOS } from '@/theme/tokens';
import { type Estado, PADRAO, salvarHabilitacao } from './contratos';
import { LinhaDaSkill } from './LinhaDaSkill';
import {
  agruparPorCategoria,
  contarLigadas,
  filtrarLinhas,
  type LinhaDeSkill,
  linhasDeSkills,
  ROTULO_DA_CATEGORIA,
} from './listaDeSkills';
import { SkillDoCondominioDrawer } from './SkillDoCondominioDrawer';
import { useExecutar } from './useExecutar';

type Props = {
  estado: Estado;
  condominio: string;
  titulos: ReadonlyMap<string, string>;
  editavel: boolean;
};

type Filtro = Parameters<typeof filtrarLinhas>[1];
type Grupo = ReturnType<typeof agruparPorCategoria>[number];

/** Tudo que cada linha da lista precisa saber, para não repassar sete props em cada nível. */
type ContextoDaLista = {
  condominio: string;
  ehPadrao: boolean;
  editavel: boolean;
  gravando: boolean;
  onAbrir: (linha: LinhaDeSkill) => void;
  onGravar: (linha: LinhaDeSkill, habilitado: boolean) => Promise<boolean>;
};

export function SkillsSecao({ estado, condominio, titulos, editavel }: Props): ReactNode {
  const [filtro, setFiltro] = useState<Filtro>({ busca: '', soQueMuda: false });
  const linhas = linhasDeSkills(estado, condominio, titulos);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      <BarraDaSecao
        linhas={linhas}
        ehPadrao={condominio === PADRAO}
        filtro={filtro}
        onFiltrar={setFiltro}
      />
      <ListaDeSkills
        estado={estado}
        condominio={condominio}
        editavel={editavel}
        linhas={filtrarLinhas(linhas, filtro)}
      />
    </Box>
  );
}

type BarraProps = {
  linhas: readonly LinhaDeSkill[];
  ehPadrao: boolean;
  filtro: Filtro;
  onFiltrar: (filtro: Filtro) => void;
};

function BarraDaSecao({ linhas, ehPadrao, filtro, onFiltrar }: BarraProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: 1.5, flexWrap: 'wrap' }}>
      <TituloEContagem {...contarLigadas(linhas)} />
      <CampoDeBusca busca={filtro.busca} onMudar={(busca) => onFiltrar({ ...filtro, busca })} />
      {!ehPadrao && (
        <SoQueMuda
          ligado={filtro.soQueMuda}
          onMudar={(soQueMuda) => onFiltrar({ ...filtro, soQueMuda })}
        />
      )}
    </Box>
  );
}

function TituloEContagem({ ligadas, total }: { ligadas: number; total: number }): ReactNode {
  return (
    <Box sx={{ flex: '1 1 260px' }}>
      <Typography variant="h5" component="h2">
        O que o concierge faz aqui
      </Typography>
      <Typography variant="body2" color="text.tertiary">
        {`${ligadas} de ${total} ligadas`}
      </Typography>
    </Box>
  );
}

const ADORNO_DA_BUSCA = {
  startAdornment: (
    <InputAdornment position="start">
      <SearchOutlined fontSize="small" />
    </InputAdornment>
  ),
};

function CampoDeBusca({ busca, onMudar }: { busca: string; onMudar: (busca: string) => void }) {
  return (
    <TextField
      size="small"
      placeholder="Buscar…"
      value={busca}
      onChange={(evento) => onMudar(evento.target.value)}
      slotProps={{ input: ADORNO_DA_BUSCA, htmlInput: { 'aria-label': 'Buscar skill' } }}
    />
  );
}

function SoQueMuda({ ligado, onMudar }: { ligado: boolean; onMudar: (ligado: boolean) => void }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Switch
        size="small"
        checked={ligado}
        onChange={(_, novo) => onMudar(novo)}
        slotProps={{ input: { 'aria-label': 'Só o que muda do Padrão' } }}
      />
      <Typography variant="body2">Só o que muda do Padrão</Typography>
    </Box>
  );
}

const ESTILO_DA_LISTA = {
  border: 1,
  borderColor: 'border.default',
  borderRadius: `${RAIOS.cartao}px`,
  bgcolor: 'background.paper',
  overflow: 'hidden',
} as const;

type ListaProps = Pick<Props, 'estado' | 'condominio' | 'editavel'> & {
  linhas: readonly LinhaDeSkill[];
};

function ListaDeSkills({ estado, condominio, editavel, linhas }: ListaProps): ReactNode {
  const [aberta, setAberta] = useState<LinhaDeSkill | null>(null);
  const contexto = useContextoDaLista({ condominio, editavel, onAbrir: setAberta });

  return (
    <>
      <Box sx={ESTILO_DA_LISTA}>
        {linhas.length === 0 && <SemSkills />}
        {agruparPorCategoria(linhas).map((grupo) => (
          <GrupoDeSkills key={grupo.categoria} grupo={grupo} contexto={contexto} />
        ))}
      </Box>
      <SkillAberta
        linha={aberta}
        estado={estado}
        contexto={contexto}
        onFechar={() => setAberta(null)}
      />
    </>
  );
}

function useContextoDaLista(
  entrada: Pick<ContextoDaLista, 'condominio' | 'editavel' | 'onAbrir'>,
): ContextoDaLista {
  const { executar, gravando } = useExecutar();
  const { condominio } = entrada;
  const onGravar = (linha: LinhaDeSkill, habilitado: boolean): Promise<boolean> => {
    const { nome } = linha.caso;
    const { papeis } = linha.habilitacao;
    return executar((escopo) =>
      salvarHabilitacao({ condominio, caso: nome, habilitado, papeis, limpar: false }, escopo),
    );
  };
  return { ...entrada, ehPadrao: condominio === PADRAO, gravando, onGravar };
}

function SemSkills(): ReactNode {
  return (
    <Typography color="text.secondary" sx={{ p: 2 }}>
      Nenhuma skill encontrada.
    </Typography>
  );
}

const ESTILO_DO_TITULO_DE_GRUPO = {
  display: 'block',
  px: 2.5,
  pt: 2,
  pb: 1,
  color: 'text.tertiary',
  fontWeight: 600,
} as const;

function GrupoDeSkills({ grupo, contexto }: { grupo: Grupo; contexto: ContextoDaLista }) {
  return (
    <Box>
      <TituloDoGrupo categoria={grupo.categoria} />
      {grupo.linhas.map((linha) => (
        <LinhaDaSkill
          key={linha.caso.nome}
          linha={linha}
          alvoDoRealce={`condominio:${contexto.condominio}:skill:${linha.caso.nome}`}
          ehPadrao={contexto.ehPadrao}
          editavel={contexto.editavel}
          gravando={contexto.gravando}
          onAbrir={() => contexto.onAbrir(linha)}
          onMudarHabilitado={(habilitado) => void contexto.onGravar(linha, habilitado)}
        />
      ))}
    </Box>
  );
}

function TituloDoGrupo({ categoria }: { categoria: Grupo['categoria'] }): ReactNode {
  return (
    <Typography variant="overline" sx={ESTILO_DO_TITULO_DE_GRUPO}>
      {ROTULO_DA_CATEGORIA[categoria]}
    </Typography>
  );
}

type SkillAbertaProps = {
  linha: LinhaDeSkill | null;
  estado: Estado;
  contexto: ContextoDaLista;
  onFechar: () => void;
};

function SkillAberta({ linha, estado, contexto, onFechar }: SkillAbertaProps): ReactNode {
  if (linha === null) return null;
  return (
    <SkillDoCondominioDrawer
      estado={estado}
      condominio={contexto.condominio}
      caso={linha.caso}
      titulo={linha.titulo}
      editavel={contexto.editavel}
      onFechar={onFechar}
    />
  );
}
