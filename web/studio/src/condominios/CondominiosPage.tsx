import {
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Link as LinkExterno,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material';
import { type ReactNode, useState } from 'react';
import { Link, useNavigate } from 'react-router';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { EmptyState } from '@/shared/ui/EmptyState';
import { paraIdentificador } from '@/skills/contratos';
import { AdministradorasAba } from './AdministradorasAba';
import { ComEstado } from './ComEstado';
import {
  type Condominio,
  type Estado,
  PADRAO,
  salvarCondominio,
  useEscopo,
  useEstado,
} from './contratos';
import { SeletorDeAdministradora } from './SeletorDeAdministradora';
import { useExecutar } from './useExecutar';

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

type NovoProps = { aberto: boolean; estado: Estado; onFechar: () => void };

function NovoCondominio({ aberto, estado, onFechar }: NovoProps): ReactNode {
  const escopo = useEscopo() ?? null;
  const { executar, gravando } = useExecutar();
  const navegar = useNavigate();
  const { mostrar } = useSnackbar();
  const [nome, setNome] = useState('');
  const [administradora, setAdministradora] = useState('');
  const donaDoNovo = escopo ?? administradora;

  async function criar(): Promise<void> {
    const id = paraIdentificador(nome);
    if (id === '') return mostrar('Use letras ou números no nome.', 'error');
    if (id === PADRAO || estado.mapeamento.condominios.some((c) => c.id === id)) {
      return mostrar('Já existe um condomínio com esse nome.', 'error');
    }
    const novo: Condominio = {
      id,
      nome: nome.trim(),
      administradora_id: donaDoNovo,
      account: '',
      company_id: '',
      tenant: '',
    };
    const criado = await executar((e) => salvarCondominio(novo, e), 'Condomínio criado.');
    if (!criado) return;
    setNome('');
    onFechar();
    void navegar(`/condominios/${encodeURIComponent(id)}`);
  }

  return (
    <Dialog open={aberto} onClose={onFechar} fullWidth maxWidth="xs">
      <DialogTitle>Novo condomínio</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
        <TextField
          autoFocus
          label="Nome"
          value={nome}
          onChange={(e) => setNome(e.target.value)}
          sx={{ mt: 1 }}
        />
        {escopo === null && (
          <SeletorDeAdministradora
            administradoras={estado.administradoras}
            valor={administradora}
            onMudar={setAdministradora}
          />
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onFechar}>Cancelar</Button>
        <Button
          variant="contained"
          disabled={gravando || nome.trim() === ''}
          onClick={() => void criar()}
        >
          Criar condomínio
        </Button>
      </DialogActions>
    </Dialog>
  );
}
