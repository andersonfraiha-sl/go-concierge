import { Box, Button, Link as LinkExterno, Tab, Tabs, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useSearchParams } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { AdministradorasAba } from './AdministradorasAba';
import { ComEstado } from './ComEstado';
import { type Estado, useEstado } from './contratos';
import { MatrizDeSkills } from './MatrizDeSkills';
import { NovoCondominio } from './NovoCondominio';
import { TabelaDeCondominios } from './TabelaDeCondominios';

type Aba = 'condominios' | 'skills' | 'administradoras';

// Temporário: o que o Studio ainda não faz fica nas telas antigas (fora do SPA).
const TELAS_ANTIGAS = [
  { rotulo: 'Skills × condomínios', destino: '/admin' },
  { rotulo: 'Renomear administradora', destino: '/backdoor/administradoras' },
  { rotulo: 'Definições livres', destino: '/backdoor/payloads' },
] as const;

function TelasAntigas(): ReactNode {
  return (
    <Box
      component="nav"
      aria-label="Tela antiga"
      sx={{ display: 'flex', gap: 1.5, typography: 'caption' }}
    >
      <Typography variant="caption" color="text.secondary">
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

type Permissoes = { matriz: boolean; administradoras: boolean };

function abaPermitida(pedida: string | null, permitido: Permissoes): Aba {
  if (pedida === 'skills' && permitido.matriz) return 'skills';
  if (pedida === 'administradoras' && permitido.administradoras) return 'administradoras';
  return 'condominios';
}

function comAba(atual: URLSearchParams, aba: Aba): URLSearchParams {
  const proximos = new URLSearchParams(atual);
  proximos.set('aba', aba);
  return proximos;
}

function usePermissoesDaPagina(): Permissoes {
  return {
    matriz: usePodeEditarCondominio('condominios.habilitar'),
    administradoras: usePodeEditarCondominio('administradoras.editar'),
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
      <CabecalhoDaPagina aba={aba} onNovo={novo.abrir} />
      <AbasDaPagina aba={aba} permitido={permitido} onTrocar={trocarAba} />
      <ComEstado consulta={consulta}>
        {(estado) => <CorpoDaPagina estado={estado} aba={aba} permitido={permitido} novo={novo} />}
      </ComEstado>
    </Box>
  );
}

function CabecalhoDaPagina({ aba, onNovo }: { aba: Aba; onNovo: () => void }) {
  const podeCriar = usePodeEditarCondominio('condominios.editar');
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: 2 }}>
      <Box sx={{ flex: 1 }}>
        <Typography variant="h5" component="h1">
          Condomínios
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Diga o que o concierge faz em cada condomínio.
        </Typography>
      </Box>
      {podeCriar && aba === 'condominios' && (
        <Button variant="contained" onClick={onNovo}>
          Novo condomínio
        </Button>
      )}
    </Box>
  );
}

type AbasProps = { aba: Aba; permitido: Permissoes; onTrocar: (aba: Aba) => void };

function AbasDaPagina({ aba, permitido, onTrocar }: AbasProps): ReactNode {
  if (!permitido.matriz && !permitido.administradoras) return null;

  return (
    <Tabs value={aba} onChange={(_, valor: Aba) => onTrocar(valor)}>
      <Tab value="condominios" label="Condomínios" />
      {permitido.matriz && <Tab value="skills" label="Visão por skill" />}
      {permitido.administradoras && <Tab value="administradoras" label="Administradoras" />}
    </Tabs>
  );
}

type CorpoProps = { estado: Estado; aba: Aba; permitido: Permissoes; novo: Novo };

function CorpoDaPagina({ estado, aba, permitido, novo }: CorpoProps): ReactNode {
  return (
    <>
      <ConteudoDaAba aba={aba} estado={estado} permitido={permitido} />
      <NovoCondominio aberto={novo.aberto} estado={estado} onFechar={novo.fechar} />
      {permitido.administradoras && <TelasAntigas />}
    </>
  );
}

type ConteudoProps = { aba: Aba; estado: Estado; permitido: Permissoes };

function ConteudoDaAba({ aba, estado, permitido }: ConteudoProps): ReactNode {
  if (aba === 'skills') {
    const titulos = new Map(estado.casos.map((caso) => [caso.nome, caso.titulo]));
    return <MatrizDeSkills estado={estado} titulos={titulos} editavel={permitido.matriz} />;
  }
  if (aba === 'administradoras') {
    return (
      <AdministradorasAba
        administradoras={estado.administradoras}
        condominios={estado.mapeamento.condominios}
      />
    );
  }
  return <TabelaDeCondominios estado={estado} />;
}
