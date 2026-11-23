import AddIcon from '@mui/icons-material/Add';
import {
  Box,
  Button,
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
import { Link as RouterLink } from 'react-router';
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
import { PESOS, RAIO, TIPOS, TOKENS_DA_SKILL } from '@/theme/tokens';
import { vemDoFluxo } from './arquivosDaSkill';

type Props = { nome: string; caminho: string };

/**
 * A mesma ferramenta pode voltar duas vezes: uma vez solta na lista da skill, outra pelo Fluxo
 * que a usa num passo. O board mostra uma linha só, a do Fluxo (sem switch, já que o Fluxo decide).
 */
function semDuplicadosDoFluxo(recursos: readonly RecursoDeclarado[]): RecursoDeclarado[] {
  const doFluxoPorNome = new Map(
    recursos.filter(vemDoFluxo).map((recurso) => [recurso.nome, recurso]),
  );
  const vistos = new Set<string>();
  return recursos.flatMap((recurso) => {
    if (vistos.has(recurso.nome)) return [];
    vistos.add(recurso.nome);
    return [doFluxoPorNome.get(recurso.nome) ?? recurso];
  });
}

export function FerramentasAba({ caminho }: Props): ReactNode {
  const agente = useAgente(caminho);
  const editavel = usePodeEditar('ferramentas.editar');
  const vincular = useVincularFerramenta(caminho);
  const recursos = semDuplicadosDoFluxo(agente.data?.recursos ?? []);

  return (
    <Box sx={{ display: 'flex', gap: 3, alignItems: 'flex-start' }}>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <SectionCard
          titulo="Ferramentas"
          ajuda="Sistemas que a skill pode usar. As que só consultam você liga aqui; as que gravam entram por um passo do Fluxo."
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
      </Box>
      <PainelComoFunciona recursos={recursos} />
    </Box>
  );
}

function PainelComoFunciona({ recursos }: { recursos: readonly RecursoDeclarado[] }): ReactNode {
  const consultas = recursos.filter((r) => r.recurso && !r.recurso.write).length;
  const gravacoes = recursos.filter((r) => r.recurso?.write).length;
  return (
    <Box
      component="aside"
      sx={{
        width: 331,
        flexShrink: 0,
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
      }}
    >
      <Typography
        component="h2"
        variant="overline"
        sx={{ color: 'text.secondary', fontWeight: PESOS.negrito }}
      >
        Como funciona
      </Typography>
      <Typography variant="h6" component="p">
        Consulta ou grava
      </Typography>
      <BlocoDeTipo
        rotulo="Só consulta"
        bgcolor="background.tint"
        color="primary.main"
        quantidade={consultas}
        texto="Liga aqui, nesta aba: a skill lê o sistema quando precisa, sem pedir confirmação."
      />
      <BlocoDeTipo
        rotulo="Grava no sistema"
        bgcolor="studio.registra.soft"
        color="studio.registra.onSoft"
        quantidade={gravacoes}
        texto="Entra por um passo do tipo Gravar no sistema, no Fluxo."
      />
      <Typography
        component={RouterLink}
        to="?aba=fluxo"
        variant="body2"
        sx={{ color: 'primary.main', textDecoration: 'none' }}
      >
        Abrir o Fluxo
      </Typography>
      <Typography
        component="h2"
        variant="overline"
        sx={{ color: 'text.secondary', fontWeight: PESOS.negrito, mt: 1 }}
      >
        Todas as ferramentas
      </Typography>
      <Typography
        component={RouterLink}
        to="/ferramentas"
        variant="body2"
        sx={{ color: 'primary.main', textDecoration: 'none' }}
      >
        Ferramentas
      </Typography>
    </Box>
  );
}

function BlocoDeTipo({
  rotulo,
  bgcolor,
  color,
  quantidade,
  texto,
}: {
  rotulo: string;
  bgcolor: string;
  color: string;
  quantidade: number;
  texto: string;
}): ReactNode {
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
      <Box
        component="span"
        sx={{
          alignSelf: 'flex-start',
          fontSize: TIPOS.caption,
          fontWeight: PESOS.medio,
          py: 0.5,
          px: 1.25,
          borderRadius: RAIO.pilula,
          bgcolor,
          color,
        }}
      >
        {rotulo} · {quantidade}
      </Box>
      <Typography variant="body2" color="textTertiary">
        {texto}
      </Typography>
    </Box>
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
    <Typography color="textSecondary">Esta skill ainda não usa nenhuma ferramenta.</Typography>
  );
}

type BotaoProps = { jaLigadas: string[]; onEscolher: (nome: string) => void };

function BotaoAdicionarFerramenta({ jaLigadas, onEscolher }: BotaoProps): ReactNode {
  const [adicionando, setAdicionando] = useState(false);
  return (
    <>
      <Button
        size="small"
        variant="outlined"
        startIcon={<AddIcon />}
        onClick={() => setAdicionando(true)}
        sx={{ height: 34, px: 1.75, borderColor: 'border.default', color: 'text.strong' }}
      >
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
  const pendente = !recurso.ok;
  return (
    <Paper
      variant="outlined"
      sx={{
        p: 1.5,
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        borderRadius: RAIO.painel,
        borderColor: pendente ? undefined : 'border.subtle',
        bgcolor: pendente ? TOKENS_DA_SKILL.pendenciaFundo : undefined,
      }}
    >
      <TextosDaFerramenta recurso={recurso} editavel={editavel} />
      {vemDoFluxo(recurso) ? (
        <TipoDeFerramenta
          rotulo="Usada no fluxo"
          bgcolor="background.neutralFill"
          color="textSecondary"
        />
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

function TipoDeFerramenta({
  rotulo,
  bgcolor,
  color,
}: {
  rotulo: string;
  bgcolor: string;
  color: string;
}): ReactNode {
  return (
    <Box
      component="span"
      sx={{
        fontSize: TIPOS.caption,
        fontWeight: PESOS.medio,
        py: 0.5,
        px: 1.25,
        borderRadius: RAIO.pilula,
        bgcolor,
        color,
        flexShrink: 0,
      }}
    >
      {rotulo}
    </Box>
  );
}

function TextosDaFerramenta({ recurso, editavel }: Omit<LinhaProps, 'onDesligar'>): ReactNode {
  const write = recurso.recurso?.write ?? false;
  return (
    <Box sx={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.5 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Typography variant="subtitle2">{recurso.recurso?.titulo ?? recurso.nome}</Typography>
        {recurso.recurso &&
          (write ? (
            <TipoDeFerramenta
              rotulo="Grava no sistema"
              bgcolor="studio.registra.soft"
              color="studio.registra.onSoft"
            />
          ) : (
            <TipoDeFerramenta rotulo="Só consulta" bgcolor="background.tint" color="primary.main" />
          ))}
      </Box>
      {editavel && recurso.recurso && (
        <Typography variant="caption" color="textSecondary">
          {recurso.recurso.description}
        </Typography>
      )}
      {!recurso.ok && recurso.problema && (
        <Typography variant="caption" color="warning" sx={{ display: 'block' }} role="alert">
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
        <Typography color="textSecondary">Não há mais ferramentas para adicionar.</Typography>
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
