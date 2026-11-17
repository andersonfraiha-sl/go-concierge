import AddOutlined from '@mui/icons-material/AddOutlined';
import { Box, Button, Link as LinkExterno, Tab, Tabs, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useSearchParams } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { useSkills } from '@/skills/contratos';
import { CONDOMINIOS, PESOS, SHELL } from '@/theme/tokens';
import { ComEstado } from './ComEstado';
import { type Estado, useEstado } from './contratos';
import { MatrizDeSkills } from './MatrizDeSkills';
import { NovoCondominio } from './NovoCondominio';
import { TabelaDeCondominios } from './TabelaDeCondominios';

type Aba = 'condominios' | 'skills';

// Temporário: o que o Studio ainda não faz fica nas telas antigas (fora do SPA).
const TELAS_ANTIGAS = [
  { rotulo: 'Skills × condomínios', destino: '/admin' },
  { rotulo: 'Definições livres', destino: '/backdoor/payloads' },
] as const;

function TelasAntigas(): ReactNode {
  return (
    <Box
      component="nav"
      aria-label="Tela antiga"
      sx={{ display: 'flex', gap: 1.5, typography: 'caption' }}
    >
      <Typography variant="caption" color="textSecondary">
        Tela antiga:
      </Typography>
      {TELAS_ANTIGAS.map((tela) => (
        <LinkExterno key={tela.destino} href={tela.destino} variant="caption">
          {tela.rotulo}
        </LinkExterno>
      ))}
    </Box>
  );
}

type Permissoes = { matriz: boolean; telasAntigas: boolean };

function abaPermitida(pedida: string | null, permitido: Permissoes): Aba {
  return pedida === 'skills' && permitido.matriz ? 'skills' : 'condominios';
}

function comAba(atual: URLSearchParams, aba: Aba): URLSearchParams {
  const proximos = new URLSearchParams(atual);
  proximos.set('aba', aba);
  return proximos;
}

function usePermissoesDaPagina(): Permissoes {
  return {
    matriz: usePodeEditarCondominio('condominios.habilitar'),
    telasAntigas: usePodeEditarCondominio('administradoras.editar'),
  };
}

function useAbaDaPagina(permitido: Permissoes): [Aba, (aba: Aba) => void] {
  const [params, setParams] = useSearchParams();
  const trocar = (aba: Aba): void => setParams((atual) => comAba(atual, aba), { replace: true });
  return [abaPermitida(params.get('aba'), permitido), trocar];
}

function useDialogoDoNovo() {
  const [aberto, setAberto] = useState(false);
  return { aberto, abrir: () => setAberto(true), fechar: () => setAberto(false) };
}

type Novo = ReturnType<typeof useDialogoDoNovo>;

export function CondominiosPage(): ReactNode {
  const consulta = useEstado();
  const permitido = usePermissoesDaPagina();
  const [aba, trocarAba] = useAbaDaPagina(permitido);
  const novo = useDialogoDoNovo();
  useContextoDaTela({ tela: 'condominios' });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <CabecalhoDaPagina onNovo={novo.abrir} />
      <AbasDaPagina aba={aba} permitido={permitido} onTrocar={trocarAba} />
      <ComEstado consulta={consulta}>
        {(estado) => <CorpoDaPagina estado={estado} aba={aba} permitido={permitido} novo={novo} />}
      </ComEstado>
    </Box>
  );
}

function CabecalhoDaPagina({ onNovo }: { onNovo: () => void }) {
  const podeCriar = usePodeEditarCondominio('condominios.editar');
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: 2 }}>
      <Box sx={{ flex: 1 }}>
        <Typography
          variant="h4"
          component="h1"
          color="secondary"
          sx={{ letterSpacing: CONDOMINIOS.letterSpacingDoTitulo }}
        >
          Condomínios
        </Typography>
        <Typography variant="subtitle1" color="textSecondary" sx={{ fontWeight: PESOS.regular }}>
          Diga o que o concierge faz em cada condomínio.
        </Typography>
      </Box>
      {podeCriar && (
        <Button variant="contained" startIcon={<AddOutlined />} onClick={onNovo}>
          Novo condomínio
        </Button>
      )}
    </Box>
  );
}

type AbasProps = { aba: Aba; permitido: Permissoes; onTrocar: (aba: Aba) => void };

function AbasDaPagina({ aba, permitido, onTrocar }: AbasProps): ReactNode {
  if (!permitido.matriz) return null;

  return (
    <Tabs
      value={aba}
      onChange={(_, valor: Aba) => onTrocar(valor)}
      sx={{
        minHeight: 0,
        '& .MuiTabs-list': { gap: `${SHELL.gapDasAbas}px` },
        '& .MuiTab-root': { minHeight: 0, minWidth: 0, px: 0, py: 1.625 },
        '& .MuiTab-root.Mui-selected': { fontWeight: PESOS.semi },
      }}
    >
      <Tab value="condominios" label="Condomínios" />
      <Tab value="skills" label="Visão por skill" />
    </Tabs>
  );
}

type CorpoProps = { estado: Estado; aba: Aba; permitido: Permissoes; novo: Novo };

function CorpoDaPagina({ estado, aba, permitido, novo }: CorpoProps): ReactNode {
  return (
    <>
      <ConteudoDaAba aba={aba} estado={estado} permitido={permitido} />
      <NovoCondominio aberto={novo.aberto} estado={estado} onFechar={novo.fechar} />
      {permitido.telasAntigas && <TelasAntigas />}
    </>
  );
}

type ConteudoProps = { aba: Aba; estado: Estado; permitido: Permissoes };

function ConteudoDaAba({ aba, estado, permitido }: ConteudoProps): ReactNode {
  if (aba === 'skills') {
    return <MatrizDeSkillsAba estado={estado} editavel={permitido.matriz} />;
  }
  return <TabelaDeCondominios estado={estado} />;
}

type MatrizAbaProps = { estado: Estado; editavel: boolean };

/** Os títulos vêm do catálogo de skills (tem nome amigável até para quem não é "caso", tipo faq/regimento). */
function MatrizDeSkillsAba({ estado, editavel }: MatrizAbaProps): ReactNode {
  const { data: skills } = useSkills();
  const titulos = new Map((skills?.skills ?? []).map((skill) => [skill.nome, skill.titulo]));
  return <MatrizDeSkills estado={estado} titulos={titulos} editavel={editavel} />;
}
