import { zodResolver } from '@hookform/resolvers/zod';
import DeleteOutlined from '@mui/icons-material/DeleteOutlined';
import { Box, Button, IconButton, Paper, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { Link } from 'react-router';
import { z } from 'zod';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { InlineTitle } from '@/shared/ui/InlineTitle';
import { paraIdentificador } from '@/skills/contratos';
import {
  type Administradora,
  type Condominio,
  criarAdministradora,
  removerAdministradora,
  salvarAdministradora,
} from './contratos';
import { useExecutar } from './useExecutar';

type Props = { administradoras: Administradora[]; condominios: Condominio[] };

const esquemaDaNova = (administradoras: Administradora[]) =>
  z.object({
    nome: z
      .string()
      .refine((nome) => paraIdentificador(nome) !== '', 'Use letras ou números no nome.')
      .refine(
        (nome) => !administradoras.some((a) => a.id === paraIdentificador(nome)),
        'Já existe uma administradora com esse nome.',
      ),
  });

/** Até 1, singular; a partir de 2, plural. */
function contagemDeCondominios(quantidade: number): string {
  if (quantidade === 0) return 'Nenhum condomínio';
  if (quantidade === 1) return '1 condomínio';
  return `${quantidade} condomínios`;
}

type Execucao = ReturnType<typeof useExecutar>;

function useRemocao({ executar }: Execucao) {
  const [aRemover, setARemover] = useState<Administradora | null>(null);
  const confirmar = async (): Promise<void> => {
    if (aRemover === null) return;
    setARemover(null);
    await executar(
      (escopo) => removerAdministradora(aRemover.id, escopo),
      'Administradora removida.',
    );
  };
  return { aRemover, pedir: setARemover, cancelar: () => setARemover(null), confirmar };
}

export function AdministradorasAba({ administradoras, condominios }: Props): ReactNode {
  const execucao = useExecutar();
  const remocao = useRemocao(execucao);

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <NovaAdministradora administradoras={administradoras} execucao={execucao} />
      <ListaDeAdministradoras
        administradoras={administradoras}
        condominios={condominios}
        execucao={execucao}
        onPedirRemocao={remocao.pedir}
      />
      <ConfirmarRemocao remocao={remocao} />
    </Box>
  );
}

function ConfirmarRemocao({ remocao }: { remocao: ReturnType<typeof useRemocao> }): ReactNode {
  return (
    <ConfirmDialog
      aberto={remocao.aRemover !== null}
      titulo="Remover administradora"
      texto={`Remover "${remocao.aRemover?.nome ?? ''}"? Não dá para desfazer.`}
      confirmar="Remover"
      onConfirmar={() => void remocao.confirmar()}
      onCancelar={remocao.cancelar}
    />
  );
}

function useNovaAdministradora(
  administradoras: Administradora[],
  { executar, gravando }: Execucao,
) {
  const { control, handleSubmit, reset } = useForm({
    resolver: zodResolver(esquemaDaNova(administradoras)),
    defaultValues: { nome: '' },
  });
  const nome = useWatch({ control, name: 'nome' });

  const criar = handleSubmit(async (valores) => {
    const id = paraIdentificador(valores.nome);
    const criada = await executar(
      (escopo) => criarAdministradora({ id, nome: valores.nome.trim() }, escopo),
      'Administradora criada.',
    );
    if (criada) reset();
  });

  return { control, criar, desabilitado: gravando || nome.trim() === '' };
}

type NovaProps = { administradoras: Administradora[]; execucao: Execucao };

function NovaAdministradora({ administradoras, execucao }: NovaProps): ReactNode {
  const { control, criar, desabilitado } = useNovaAdministradora(administradoras, execucao);

  return (
    <Box
      component="form"
      onSubmit={(e) => void criar(e)}
      sx={{ display: 'flex', gap: 1, alignItems: 'center' }}
    >
      <CampoTexto control={control} name="nome" size="small" label="Nome da administradora" />
      <Button type="submit" variant="contained" disabled={desabilitado}>
        Criar administradora
      </Button>
    </Box>
  );
}

type ListaProps = {
  administradoras: Administradora[];
  condominios: Condominio[];
  execucao: Execucao;
  onPedirRemocao: (administradora: Administradora) => void;
};

function ListaDeAdministradoras({ administradoras, condominios, ...resto }: ListaProps): ReactNode {
  if (administradoras.length === 0) return <EmptyState frase="Nenhuma administradora ainda." />;

  return (
    <Paper variant="outlined" component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
      {administradoras.map((administradora) => (
        <LinhaDaAdministradora
          key={administradora.id}
          administradora={administradora}
          doCondominio={condominios.filter((c) => c.administradora_id === administradora.id)}
          {...resto}
        />
      ))}
    </Paper>
  );
}

const ESTILO_DA_LINHA = {
  display: 'flex',
  alignItems: 'center',
  gap: 1.5,
  px: 2,
  py: 1,
  borderBottom: 1,
  borderColor: 'divider',
  '&:last-child': { borderBottom: 0 },
} as const;

type LinhaProps = {
  administradora: Administradora;
  doCondominio: Condominio[];
  execucao: Execucao;
  onPedirRemocao: (administradora: Administradora) => void;
};

function LinhaDaAdministradora({ administradora, doCondominio, ...acoes }: LinhaProps): ReactNode {
  const { gravando } = acoes.execucao;

  return (
    <Box component="li" sx={ESTILO_DA_LINHA}>
      <NomeEId administradora={administradora} execucao={acoes.execucao} />
      <Box sx={{ flex: 1 }} />
      <ResumoDeCondominios administradoraId={administradora.id} quantidade={doCondominio.length} />
      <IconButton
        aria-label={`Remover ${administradora.nome}`}
        disabled={gravando || doCondominio.length > 0}
        onClick={() => acoes.onPedirRemocao(administradora)}
      >
        <DeleteOutlined />
      </IconButton>
    </Box>
  );
}

type NomeEIdProps = { administradora: Administradora; execucao: Execucao };

function NomeEId({ administradora, execucao: { executar, gravando } }: NomeEIdProps): ReactNode {
  const renomear = (nome: string): Promise<boolean> =>
    executar(
      (escopo) => salvarAdministradora({ id: administradora.id, nome }, escopo),
      'Administradora renomeada.',
    );

  return (
    <>
      <InlineTitle
        valor={administradora.nome}
        editavel={!gravando}
        rotulo={`nome da administradora ${administradora.nome}`}
        component="span"
        onMudar={(nome) => void renomear(nome)}
      />
      <IdDaAdministradora id={administradora.id} />
    </>
  );
}

function IdDaAdministradora({ id }: { id: string }): ReactNode {
  return (
    <Typography variant="caption" color="text.secondary">
      {id}
    </Typography>
  );
}

type ResumoProps = { administradoraId: string; quantidade: number };

function ResumoDeCondominios({ administradoraId, quantidade }: ResumoProps): ReactNode {
  return (
    <>
      <LinkDosCondominios administradoraId={administradoraId} quantidade={quantidade} />
      {quantidade > 0 && (
        <Typography variant="caption" color="text.secondary">
          Mova os condomínios dela antes de remover.
        </Typography>
      )}
    </>
  );
}

function LinkDosCondominios({ administradoraId, quantidade }: ResumoProps): ReactNode {
  const destino = `/condominios?aba=condominios&administradora=${encodeURIComponent(administradoraId)}`;
  return (
    <Typography component={Link} to={destino} variant="body2" color="primary">
      {contagemDeCondominios(quantidade)}
    </Typography>
  );
}
