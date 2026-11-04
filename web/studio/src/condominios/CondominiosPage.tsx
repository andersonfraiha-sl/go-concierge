import { Box, Button, Link as LinkExterno, Tab, Tabs, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useSearchParams } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { AdministradorasAba } from './AdministradorasAba';
import { ComEstado } from './ComEstado';
import { useEstado } from './contratos';
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

export function CondominiosPage(): ReactNode {
  const consulta = useEstado();
  const podeCriar = usePodeEditarCondominio('condominios.editar');
  const podeVerMatriz = usePodeEditarCondominio('condominios.habilitar');
  const podeVerAdministradoras = usePodeEditarCondominio('administradoras.editar');
  const [params, setParams] = useSearchParams();
  const [criando, setCriando] = useState(false);

  const abaDaQuery = params.get('aba');
  const aba: Aba =
    abaDaQuery === 'skills' && podeVerMatriz
      ? 'skills'
      : abaDaQuery === 'administradoras' && podeVerAdministradoras
        ? 'administradoras'
        : 'condominios';

  const trocarAba = (valor: Aba): void =>
    setParams(
      (atual) => {
        const proximos = new URLSearchParams(atual);
        proximos.set('aba', valor);
        return proximos;
      },
      { replace: true },
    );

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
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
          <Button variant="contained" onClick={() => setCriando(true)}>
            Novo condomínio
          </Button>
        )}
      </Box>

      {(podeVerMatriz || podeVerAdministradoras) && (
        <Tabs value={aba} onChange={(_, valor: Aba) => trocarAba(valor)}>
          <Tab value="condominios" label="Condomínios" />
          {podeVerMatriz && <Tab value="skills" label="Visão por skill" />}
          {podeVerAdministradoras && <Tab value="administradoras" label="Administradoras" />}
        </Tabs>
      )}

      <ComEstado consulta={consulta}>
        {(estado) => (
          <>
            {aba === 'condominios' && <TabelaDeCondominios estado={estado} />}
            {aba === 'skills' && (
              <MatrizDeSkills
                estado={estado}
                titulos={new Map(estado.casos.map((caso) => [caso.nome, caso.titulo]))}
                editavel={podeVerMatriz}
              />
            )}
            {aba === 'administradoras' && (
              <AdministradorasAba
                administradoras={estado.administradoras}
                condominios={estado.mapeamento.condominios}
              />
            )}
            <NovoCondominio aberto={criando} estado={estado} onFechar={() => setCriando(false)} />
            {podeVerAdministradoras && <TelasAntigas />}
          </>
        )}
      </ComEstado>
    </Box>
  );
}
