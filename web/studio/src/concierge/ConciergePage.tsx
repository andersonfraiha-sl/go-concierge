import { Box, Button, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useState } from 'react';
import { useCopiloto } from '@/copiloto/CopilotoProvider';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { RegistroDeSalvamento } from '@/shared/autosave/registroDeSalvamento';
import { formatarHora } from '@/shared/autosave/SaveIndicator';
import { useSkills } from '@/skills/contratos';
import { CONCIERGE, PESOS, RAIO, SHELL, TIPOS } from '@/theme/tokens';
import { BaseDeConhecimentoCard } from './BaseDeConhecimentoCard';
import { BoasVindasAba, TransbordoAba } from './BoasVindasAba';
import { IdentidadeCard } from './IdentidadeCard';
import { Falha, PersonaDoAgente } from './PersonaAba';
import { RegrasGeraisCard } from './RegrasGeraisCard';

const ESTILO_DA_GRADE = {
  display: 'grid',
  gridTemplateColumns: 'minmax(0,1.45fr) minmax(0,1fr)',
  gap: 3,
} as const;

const ESTILO_DA_COLUNA = { display: 'flex', flexDirection: 'column', gap: 3 } as const;

export function ConciergePage(): ReactNode {
  useContextoDaTela({ tela: 'concierge' });
  const [salvoEm, setSalvoEm] = useState<Date | null>(null);
  return (
    <RegistroDeSalvamento value={setSalvoEm}>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
        <Cabecalho salvoEm={salvoEm} />
        <ConteudoDoConcierge />
      </Box>
    </RegistroDeSalvamento>
  );
}

function Cabecalho({ salvoEm }: { salvoEm: Date | null }): ReactNode {
  const pendentes = useCopiloto().mensagens.flatMap((mensagem) => mensagem.propostas).length;
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 3 }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography
          variant="h4"
          component="h1"
          sx={{ color: 'secondary.main', letterSpacing: CONCIERGE.letterSpacingDoTitulo }}
        >
          Concierge
        </Typography>
        <Typography variant="body1" color="text.secondary" sx={{ maxWidth: 520 }}>
          Quem conversa com o morador. O que você define aqui vale para todas as skills e
          condomínios.
        </Typography>
      </Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap' }}>
        {pendentes > 0 && <IndicadorDePendencia qtd={pendentes} />}
        {salvoEm && <SalvoAs em={salvoEm} />}
        <TestarNoConsole />
      </Box>
    </Box>
  );
}

function IndicadorDePendencia({ qtd }: { qtd: number }): ReactNode {
  const frase = qtd === 1 ? '1 mudança esperando você' : `${qtd} mudanças esperando você`;
  return (
    <Box
      role="status"
      sx={{
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        gap: 0.75,
        fontSize: TIPOS.caption,
        color: 'warning.onSoft',
      }}
    >
      <Box
        sx={{
          width: SHELL.pontoDeSalvo,
          height: SHELL.pontoDeSalvo,
          borderRadius: RAIO.pilula,
          bgcolor: CONCIERGE.pontoDePendencia,
        }}
      />
      {frase}
    </Box>
  );
}

function SalvoAs({ em }: { em: Date }): ReactNode {
  return (
    <Box
      role="status"
      sx={{
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        gap: 0.75,
        fontSize: TIPOS.caption,
        color: 'success.onSoft',
      }}
    >
      <Box
        sx={{
          width: SHELL.pontoDeSalvo,
          height: SHELL.pontoDeSalvo,
          borderRadius: RAIO.pilula,
          bgcolor: 'success.main',
        }}
      />
      Salvo às {formatarHora(em)}
    </Box>
  );
}

// O Console é a tela legada fora do SPA: navegação completa, sem o basename.
function TestarNoConsole(): ReactNode {
  return (
    <Button
      component="a"
      href="/console"
      sx={{
        flexShrink: 0,
        height: SHELL.alturaDoBotaoDoDetalhe,
        px: `${SHELL.paddingDoBotaoDeTeste}px`,
        border: 1,
        borderColor: 'border.default',
        borderRadius: RAIO.chip,
        bgcolor: 'background.paper',
        color: 'text.strong',
        fontSize: TIPOS.body2,
        fontWeight: PESOS.medio,
        lineHeight: 'normal',
      }}
    >
      Testar no Console
    </Button>
  );
}

function ConteudoDoConcierge(): ReactNode {
  const skills = useSkills();
  if (skills.isPending) return <Typography color="text.secondary">Carregando…</Typography>;
  if (skills.isError) return <Falha aoTentar={() => void skills.refetch()} />;
  const assistente = skills.data.assistente;
  return (
    <Box sx={ESTILO_DA_GRADE}>
      <Box sx={ESTILO_DA_COLUNA}>
        <IdentidadeCard assistente={assistente} />
        <PersonaDoAgente assistente={assistente} />
        <BoasVindasAba assistente={assistente} />
      </Box>
      <Box sx={ESTILO_DA_COLUNA}>
        <RegrasGeraisCard assistente={assistente} />
        <BaseDeConhecimentoCard assistente={assistente} />
        <TransbordoAba assistente={assistente} />
      </Box>
    </Box>
  );
}
