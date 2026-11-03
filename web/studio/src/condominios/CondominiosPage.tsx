import {
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Link as LinkExterno,
  Tab,
  Tabs,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import { Link } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { EmptyState } from '@/shared/ui/EmptyState';
import { AdministradorasAba } from './AdministradorasAba';
import { ComEstado } from './ComEstado';
import { type Condominio, type Estado, PADRAO, useEstado } from './contratos';
import { NovoCondominio } from './NovoCondominio';

type Aba = 'condominios' | 'administradoras';

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
  const podeVerAdministradoras = usePodeEditarCondominio('administradoras.editar');
  const [aba, setAba] = useState<Aba>('condominios');
  const [criando, setCriando] = useState(false);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: 2 }}>
        <Box sx={{ flex: 1 }}>
          <Typography variant="h5" component="h1">
            Condomínios
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Diga o que a assistente faz em cada condomínio.
          </Typography>
        </Box>
        {podeCriar && aba === 'condominios' && (
          <Button variant="contained" onClick={() => setCriando(true)}>
            Novo condomínio
          </Button>
        )}
      </Box>

      {podeVerAdministradoras && (
        <Tabs value={aba} onChange={(_, valor: Aba) => setAba(valor)}>
          <Tab value="condominios" label="Condomínios" />
          <Tab value="administradoras" label="Administradoras" />
        </Tabs>
      )}

      <ComEstado consulta={consulta}>
        {(estado) => (
          <>
            {aba === 'condominios' ? (
              <ListaDeCondominios estado={estado} />
            ) : (
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

function ListaDeCondominios({ estado }: { estado: Estado }): ReactNode {
  const { condominios } = estado.mapeamento;
  const donaDe = (condominio: Condominio): string =>
    estado.administradoras.find((a) => a.id === condominio.administradora_id)?.nome ??
    'Sem administradora';

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box
        sx={{
          display: 'grid',
          gap: 2,
          gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
        }}
      >
        <CartaoDeCondominio
          id={PADRAO}
          titulo="Padrão (todos)"
          legenda="Vale para todo condomínio que não tem decisão própria."
          destaque
        />
        {condominios.map((c) => (
          <CartaoDeCondominio key={c.id} id={c.id} titulo={c.nome} legenda={donaDe(c)} />
        ))}
      </Box>
      {condominios.length === 0 && <EmptyState frase="Nenhum condomínio ainda." />}
    </Box>
  );
}

type CartaoProps = { id: string; titulo: string; legenda: string; destaque?: boolean };

function CartaoDeCondominio({ id, titulo, legenda, destaque = false }: CartaoProps): ReactNode {
  return (
    <Card
      variant="outlined"
      sx={{ borderLeft: destaque ? 4 : 1, borderLeftColor: destaque ? 'primary.main' : 'divider' }}
    >
      <CardActionArea component={Link} to={`/condominios/${encodeURIComponent(id)}`}>
        <CardContent>
          <Typography variant="subtitle1" component="h2">
            {titulo}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {legenda}
          </Typography>
        </CardContent>
      </CardActionArea>
    </Card>
  );
}
