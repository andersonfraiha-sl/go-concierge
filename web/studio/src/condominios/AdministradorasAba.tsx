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

export function AdministradorasAba({ administradoras, condominios }: Props): ReactNode {
  const { executar, gravando } = useExecutar();
  const condominiosDe = (a: Administradora): Condominio[] =>
    condominios.filter((c) => c.administradora_id === a.id);
  const [aRemover, setARemover] = useState<Administradora | null>(null);
  const { control, handleSubmit, reset } = useForm({
    resolver: zodResolver(esquemaDaNova(administradoras)),
    defaultValues: { nome: '' },
  });
  const nome = useWatch({ control, name: 'nome' });

  const criar = handleSubmit(async ({ nome }) => {
    const criada = await executar(
      (escopo) => salvarAdministradora({ id: paraIdentificador(nome), nome: nome.trim() }, escopo),
      'Administradora criada.',
    );
    if (criada) reset();
  });

  async function remover(administradora: Administradora): Promise<void> {
    setARemover(null);
    await executar(
      (escopo) => removerAdministradora(administradora.id, escopo),
      'Administradora removida.',
    );
  }

  async function renomear(administradora: Administradora, nome: string): Promise<void> {
    await executar(
      (escopo) => salvarAdministradora({ id: administradora.id, nome }, escopo),
      'Administradora renomeada.',
    );
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box
        component="form"
        onSubmit={(e) => void criar(e)}
        sx={{ display: 'flex', gap: 1, alignItems: 'center' }}
      >
        <CampoTexto control={control} name="nome" size="small" label="Nome da administradora" />
        <Button type="submit" variant="contained" disabled={gravando || nome.trim() === ''}>
          Criar administradora
        </Button>
      </Box>

      {administradoras.length === 0 ? (
        <EmptyState frase="Nenhuma administradora ainda." />
      ) : (
        <Paper variant="outlined" component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
          {administradoras.map((a) => {
            const doCondominio = condominiosDe(a);
            return (
              <Box
                component="li"
                key={a.id}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1.5,
                  px: 2,
                  py: 1,
                  borderBottom: 1,
                  borderColor: 'divider',
                  '&:last-child': { borderBottom: 0 },
                }}
              >
                <InlineTitle
                  valor={a.nome}
                  editavel={!gravando}
                  rotulo={`nome da administradora ${a.nome}`}
                  component="span"
                  onMudar={(nome) => void renomear(a, nome)}
                />
                <Typography variant="caption" color="text.secondary">
                  {a.id}
                </Typography>
                <Box sx={{ flex: 1 }} />
                <Typography
                  component={Link}
                  to={`/condominios?aba=condominios&administradora=${encodeURIComponent(a.id)}`}
                  variant="body2"
                  color="primary"
                >
                  {contagemDeCondominios(doCondominio.length)}
                </Typography>
                {doCondominio.length > 0 && (
                  <Typography variant="caption" color="text.secondary">
                    Mova os condomínios dela antes de remover.
                  </Typography>
                )}
                <IconButton
                  aria-label={`Remover ${a.nome}`}
                  disabled={gravando || doCondominio.length > 0}
                  onClick={() => setARemover(a)}
                >
                  <DeleteOutlined />
                </IconButton>
              </Box>
            );
          })}
        </Paper>
      )}

      <ConfirmDialog
        aberto={aRemover !== null}
        titulo="Remover administradora"
        texto={`Remover "${aRemover?.nome ?? ''}"? Não dá para desfazer.`}
        confirmar="Remover"
        onConfirmar={() => aRemover && void remover(aRemover)}
        onCancelar={() => setARemover(null)}
      />
    </Box>
  );
}
