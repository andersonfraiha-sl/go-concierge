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
  const vincular = useVincularFerramenta(caminho);
  const recursos = agente.data?.recursos ?? [];

  return (
    <SectionCard
      titulo="Ferramentas"
      ajuda="Sistemas que a skill pode consultar. As que gravam entram pelo Fluxo."
      acao={
        editavel && (
          <BotaoAdicionarFerramenta
            jaLigadas={recursos.map((recurso) => recurso.nome)}
            onEscolher={(nome) => void vincular(nome, true)}
          />
        )
      }
    >
      <EstadoDasFerramentas agente={agente} semRecursos={recursos.length === 0} />
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
    </SectionCard>
  );
}

function useVincularFerramenta(caminho: string) {
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();
  return async (nome: string, ligar: boolean): Promise<void> => {
    try {
      await vincularFerramenta({ agente: caminho, nome, ligar });
      await cliente.invalidateQueries({ queryKey: chaveDoAgente(caminho) });
    } catch (erro) {
      mostrar(mensagemDe(erro), 'error');
    }
  };
}

type PropsDoEstado = { agente: ReturnType<typeof useAgente>; semRecursos: boolean };

function EstadoDasFerramentas({ agente, semRecursos }: PropsDoEstado): ReactNode {
  if (agente.isPending) return <Typography variant="caption">Carregando…</Typography>;
  if (agente.isError) {
    return (
      <ErroComRetry
        mensagem="Não foi possível carregar as ferramentas."
        onTentarDeNovo={() => void agente.refetch()}
      />
    );
  }
  if (!semRecursos) return null;
  return (
    <Typography color="text.secondary">Esta skill ainda não usa nenhuma ferramenta.</Typography>
  );
}

type BotaoProps = { jaLigadas: string[]; onEscolher: (nome: string) => void };

function BotaoAdicionarFerramenta({ jaLigadas, onEscolher }: BotaoProps): ReactNode {
  const [adicionando, setAdicionando] = useState(false);
  return (
    <>
      <Button size="small" startIcon={<AddIcon />} onClick={() => setAdicionando(true)}>
        Adicionar ferramenta
      </Button>
      {adicionando && (
        <AdicionarFerramenta
          jaLigadas={jaLigadas}
          onEscolher={(nome) => {
            setAdicionando(false);
            onEscolher(nome);
          }}
          onFechar={() => setAdicionando(false)}
        />
      )}
    </>
  );
}

type LinhaProps = { recurso: RecursoDeclarado; editavel: boolean; onDesligar: () => void };

function LinhaDeFerramenta({ recurso, editavel, onDesligar }: LinhaProps): ReactNode {
  return (
    <Paper variant="outlined" sx={{ p: 1.5, display: 'flex', alignItems: 'center', gap: 1 }}>
      <TextosDaFerramenta recurso={recurso} editavel={editavel} />
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

function TextosDaFerramenta({ recurso, editavel }: Omit<LinhaProps, 'onDesligar'>): ReactNode {
  return (
    <Box sx={{ flex: 1, minWidth: 0 }}>
      <Typography variant="subtitle2">{recurso.recurso?.titulo ?? recurso.nome}</Typography>
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
  );
}

type AdicionarProps = {
  jaLigadas: string[];
  onEscolher: (nome: string) => void;
  onFechar: () => void;
};

function AdicionarFerramenta({ jaLigadas, onEscolher, onFechar }: AdicionarProps): ReactNode {
  return (
    <Dialog open fullWidth maxWidth="xs" onClose={onFechar}>
      <DialogTitle>Adicionar ferramenta</DialogTitle>
      <DialogContent>
        <FerramentasDisponiveis jaLigadas={jaLigadas} onEscolher={onEscolher} />
      </DialogContent>
      <DialogActions>
        <Button onClick={onFechar}>Cancelar</Button>
      </DialogActions>
    </Dialog>
  );
}

function FerramentasDisponiveis({ jaLigadas, onEscolher }: BotaoProps): ReactNode {
  const ferramentas = useFerramentas();
  const disponiveis = (ferramentas.data?.recursos ?? []).filter(
    (ferramenta: Ferramenta) => !ferramenta.write && !jaLigadas.includes(ferramenta.name),
  );
  return (
    <>
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
    </>
  );
}
