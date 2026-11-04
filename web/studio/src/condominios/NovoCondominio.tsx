import { zodResolver } from '@hookform/resolvers/zod';
import { Button, Dialog, DialogActions, DialogContent, DialogTitle } from '@mui/material';
import type { ReactNode } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { useNavigate } from 'react-router';
import { z } from 'zod';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { paraIdentificador } from '@/skills/contratos';
import { type Condominio, criarCondominio, type Estado, PADRAO, useEscopo } from './contratos';
import { SeletorDeAdministradora } from './SeletorDeAdministradora';
import { useExecutar } from './useExecutar';

type NovoProps = { aberto: boolean; estado: Estado; onFechar: () => void };

const esquemaDoNovo = (estado: Estado) =>
  z.object({
    nome: z
      .string()
      .refine((nome) => paraIdentificador(nome) !== '', 'Use letras ou números no nome.')
      .refine((nome) => {
        const id = paraIdentificador(nome);
        return id !== PADRAO && !estado.mapeamento.condominios.some((c) => c.id === id);
      }, 'Já existe um condomínio com esse nome.'),
    administradora: z.string(),
  });

export function NovoCondominio({ aberto, estado, onFechar }: NovoProps): ReactNode {
  const escopo = useEscopo() ?? null;
  const { executar, gravando } = useExecutar();
  const navegar = useNavigate();
  const { control, handleSubmit, reset } = useForm({
    resolver: zodResolver(esquemaDoNovo(estado)),
    defaultValues: { nome: '', administradora: '' },
  });
  const nome = useWatch({ control, name: 'nome' });

  const criar = handleSubmit(async ({ nome, administradora }) => {
    const id = paraIdentificador(nome);
    const novo: Condominio = {
      id,
      nome: nome.trim(),
      administradora_id: escopo ?? administradora,
      account: '',
      company_id: '',
      tenant: '',
    };
    const criado = await executar((e) => criarCondominio(novo, e), 'Condomínio criado.');
    if (!criado) return;
    reset();
    onFechar();
    void navegar(`/condominios/${encodeURIComponent(id)}`);
  });

  return (
    <Dialog open={aberto} onClose={onFechar} fullWidth maxWidth="xs">
      <DialogTitle>Novo condomínio</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
        <CampoTexto control={control} name="nome" autoFocus label="Nome" sx={{ mt: 1 }} />
        {escopo === null && (
          <Controller
            control={control}
            name="administradora"
            render={({ field }) => (
              <SeletorDeAdministradora
                administradoras={estado.administradoras}
                valor={field.value}
                onMudar={field.onChange}
              />
            )}
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
