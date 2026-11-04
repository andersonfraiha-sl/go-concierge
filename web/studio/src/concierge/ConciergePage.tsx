import { Box, Tab, Tabs, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useSearchParams } from 'react-router';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { ConhecimentoAba } from '@/skills/abas/ConhecimentoAba';
import { RegrasAba } from '@/skills/abas/RegrasAba';
import { useSkills } from '@/skills/contratos';
import { BoasVindasAba, TransbordoAba } from './BoasVindasAba';
import { Falha, PersonaDoAgente } from './PersonaAba';

type Aba = 'persona' | 'boas-vindas' | 'regras' | 'conhecimento' | 'transbordo';

const ABAS: ReadonlyArray<{ valor: Aba; rotulo: string }> = [
  { valor: 'persona', rotulo: 'Persona' },
  { valor: 'boas-vindas', rotulo: 'Boas-vindas' },
  { valor: 'regras', rotulo: 'Regras gerais' },
  { valor: 'conhecimento', rotulo: 'Base de conhecimento comum' },
  { valor: 'transbordo', rotulo: 'Transbordo' },
];

const abaDaConsulta = (valor: string | null): Aba =>
  ABAS.find((aba) => aba.valor === valor)?.valor ?? 'persona';

const CONTEUDO_DA_ABA: Record<Aba, (assistente: string) => ReactNode> = {
  persona: (assistente) => <PersonaDoAgente assistente={assistente} />,
  'boas-vindas': (assistente) => <BoasVindasAba assistente={assistente} />,
  regras: (assistente) => <RegrasAba caminho={assistente} titulo="Regras gerais" />,
  conhecimento: (assistente) => (
    <ConhecimentoAba caminho={assistente} titulo="Base de conhecimento comum" />
  ),
  transbordo: (assistente) => <TransbordoAba assistente={assistente} />,
};

export function ConciergePage(): ReactNode {
  const skills = useSkills();
  const [params, setParams] = useSearchParams();
  const aba = abaDaConsulta(params.get('aba'));
  useContextoDaTela({ tela: 'concierge' });

  const trocarAba = (valor: Aba): void =>
    setParams(
      (atuais) => {
        const proximos = new URLSearchParams(atuais);
        proximos.set('aba', valor);
        return proximos;
      },
      { replace: true },
    );

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Typography variant="h5" component="h1">
        Concierge
      </Typography>
      <Tabs
        value={aba}
        onChange={(_, valor: Aba) => trocarAba(valor)}
        aria-label="Seções do Concierge"
      >
        {ABAS.map(({ valor, rotulo }) => (
          <Tab key={valor} value={valor} label={rotulo} />
        ))}
      </Tabs>
      {skills.isPending && <Typography color="text.secondary">Carregando…</Typography>}
      {skills.isError && <Falha aoTentar={() => void skills.refetch()} />}
      {skills.data && CONTEUDO_DA_ABA[aba](skills.data.assistente)}
    </Box>
  );
}
