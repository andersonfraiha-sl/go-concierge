import AddIcon from '@mui/icons-material/Add';
import {
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  List,
  ListItemButton,
  ListItemText,
  Paper,
  Switch,
  Typography,
} from '@mui/material';
import { useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useState } from 'react';
import { usePodeEditar } from '@/acesso/eu';
import {
  chaveDoAgente,
  type Ferramenta,
  type RecursoDeclarado,
  useAgente,
  useFerramentas,
  vincularFerramenta,
} from '@/catalogo/contratos';
import { mensagemDe } from '@/shared/api/mensagemDe';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { SectionCard } from '@/shared/ui/SectionCard';
import { vemDoFluxo } from './arquivosDaSkill';

type Props = { nome: string; caminho: string };

export function FerramentasAba({ caminho }: Props): ReactNode {
  const agente = useAgente(caminho);
  const editavel = usePodeEditar('ferramentas.editar');
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();
  const [adicionando, setAdicionando] = useState(false);
  const recursos = agente.data?.recursos ?? [];

  const vincular = async (nome: string, ligar: boolean): Promise<void> => {
    try {
      await vincularFerramenta({ agente: caminho, nome, ligar });
      await cliente.invalidateQueries({ queryKey: chaveDoAgente(caminho) });
    } catch (erro) {
      mostrar(mensagemDe(erro), 'error');
    }
  };

  const botaoAdicionar = editavel && (
    <Button size="small" startIcon={<AddIcon />} onClick={() => setAdicionando(true)}>
      Adicionar ferramenta
    </Button>
  );

  return (
    <SectionCard
      titulo="Ferramentas"
      ajuda="Sistemas que a skill pode consultar. As que gravam entram pelo Fluxo."
      acao={botaoAdicionar}
    >
      {agente.isPending && <Typography variant="caption">Carregando…</Typography>}
      {agente.isError && (
        <ErroComRetry
          mensagem="Não foi possível carregar as ferramentas."
          onTentarDeNovo={() => void agente.refetch()}
        />
      )}
      {agente.isSuccess && recursos.length === 0 && (
        <Typography color="text.secondary">Esta skill ainda não usa nenhuma ferramenta.</Typography>
      )}
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {recursos.map((recurso) => (
          <LinhaDeFerramenta
            key={recurso.nome}
            recurso={recurso}
            editavel={editavel}
            onDesligar={() => void vincular(recurso.nome, false)}
          />
        ))}
      </Box>
      {adicionando && (
        <AdicionarFerramenta
          jaLigadas={recursos.map((recurso) => recurso.nome)}
          onEscolher={(nome) => {
            setAdicionando(false);
            void vincular(nome, true);
          }}
          onFechar={() => setAdicionando(false)}
        />
      )}
    </SectionCard>
  );
}

type LinhaProps = { recurso: RecursoDeclarado; editavel: boolean; onDesligar: () => void };

function LinhaDeFerramenta({ recurso, editavel, onDesligar }: LinhaProps): ReactNode {
  const titulo = recurso.recurso?.titulo ?? recurso.nome;
  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1 }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="subtitle2">{titulo}</Typography>
        {editavel && recurso.recurso && (
          <Typography variant="caption" color="text.secondary">
            {recurso.recurso.description}
          </Typography>
        )}
        {!recurso.ok && recurso.problema && (
          <Typography variant="caption" color="warning.main" sx={{ display: 'block' }} role="alert">
            {recurso.problema}
          </Typography>
        )}
      </Box>
      {vemDoFluxo(recurso) ? (
        <Chip size="small" variant="outlined" label="Usada no fluxo" />
      ) : (
        <Switch
          checked
          disabled={!editavel}
          onChange={onDesligar}
          slotProps={{ input: { 'aria-label': `Usar ${recurso.nome}` } }}
        />
      )}
    </Paper>
  );
}

type AdicionarProps = {
  jaLigadas: string[];
  onEscolher: (nome: string) => void;
  onFechar: () => void;
};

function AdicionarFerramenta({ jaLigadas, onEscolher, onFechar }: AdicionarProps): ReactNode {
  const ferramentas = useFerramentas();
  const disponiveis = (ferramentas.data?.recursos ?? []).filter(
    (ferramenta: Ferramenta) => !ferramenta.write && !jaLigadas.includes(ferramenta.name),
  );

  return (
    <Dialog open fullWidth maxWidth="xs" onClose={onFechar}>
      <DialogTitle>Adicionar ferramenta</DialogTitle>
      <DialogContent>
        {ferramentas.isPending && <Typography variant="caption">Carregando…</Typography>}
        {ferramentas.isError && (
          <ErroComRetry
            mensagem="Não foi possível carregar a lista."
            onTentarDeNovo={() => void ferramentas.refetch()}
          />
        )}
        {ferramentas.isSuccess && disponiveis.length === 0 && (
          <Typography color="text.secondary">Não há mais ferramentas para adicionar.</Typography>
        )}
        <List>
          {disponiveis.map((ferramenta) => (
            <ListItemButton key={ferramenta.name} onClick={() => onEscolher(ferramenta.name)}>
              <ListItemText
                primary={ferramenta.titulo ?? ferramenta.name}
                secondary={ferramenta.description}
              />
            </ListItemButton>
          ))}
        </List>
      </DialogContent>
      <DialogActions>
        <Button onClick={onFechar}>Cancelar</Button>
      </DialogActions>
    </Dialog>
  );
}
