import DeleteOutlined from '@mui/icons-material/DeleteOutlined';
import { Box, Button, IconButton, Paper, TextField, Typography } from '@mui/material';
import { useState, type FormEvent, type ReactNode } from 'react';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { EmptyState } from '@/shared/ui/EmptyState';
import { paraIdentificador } from '@/skills/contratos';
import { removerAdministradora, salvarAdministradora, type Administradora, type Condominio } from './contratos';
import { useExecutar } from './useExecutar';

type Props = { administradoras: Administradora[]; condominios: Condominio[] };

export function AdministradorasAba({ administradoras, condominios }: Props): ReactNode {
  const { executar, gravando } = useExecutar();
  const temCondominios = (a: Administradora): boolean => condominios.some((c) => c.administradora_id === a.id);
  const { mostrar } = useSnackbar();
  const [nome, setNome] = useState('');
  const [aRemover, setARemover] = useState<Administradora | null>(null);

  async function criar(evento: FormEvent): Promise<void> {
    evento.preventDefault();
    const id = paraIdentificador(nome);
    if (id === '') return mostrar('Use letras ou números no nome.', 'error');
    if (administradoras.some((a) => a.id === id)) return mostrar('Já existe uma administradora com esse nome.', 'error');
    const criada = await executar((escopo) => salvarAdministradora({ id, nome: nome.trim() }, escopo), 'Administradora criada.');
    if (criada) setNome('');
  }

  async function remover(administradora: Administradora): Promise<void> {
    setARemover(null);
    await executar((escopo) => removerAdministradora(administradora.id, escopo), 'Administradora removida.');
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box component="form" onSubmit={(e) => void criar(e)} sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
        <TextField size="small" label="Nome da administradora" value={nome} onChange={(e) => setNome(e.target.value)} />
        <Button type="submit" variant="contained" disabled={gravando || nome.trim() === ''}>
          Criar administradora
        </Button>
      </Box>

      {administradoras.length === 0 ? (
        <EmptyState frase="Nenhuma administradora ainda." />
      ) : (
        <Paper variant="outlined" component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
          {administradoras.map((a) => (
            <Box
              component="li"
              key={a.id}
              sx={{ display: 'flex', alignItems: 'center', px: 2, py: 1, borderBottom: 1, borderColor: 'divider', '&:last-child': { borderBottom: 0 } }}
            >
              <Typography sx={{ flex: 1 }}>{a.nome}</Typography>
              {temCondominios(a) && (
                <Typography variant="caption" color="text.secondary">
                  Mova os condomínios dela antes de remover.
                </Typography>
              )}
              <IconButton
                aria-label={`Remover ${a.nome}`}
                disabled={gravando || temCondominios(a)}
                onClick={() => setARemover(a)}
              >
                <DeleteOutlined />
              </IconButton>
            </Box>
          ))}
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
