import { zodResolver } from '@hookform/resolvers/zod';
import { Button, Dialog, DialogActions, DialogContent, DialogTitle } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller, useForm, useWatch } from 'react-hook-form';
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

type Valores = { nome: string; administradora: string };

function condominioNovo({ nome, administradora }: Valores, escopo: string | null): Condominio {
  const administradoraId = escopo ?? administradora;
  const semIds = { account: '', company_id: '', tenant: '' };
  return {
    id: paraIdentificador(nome),
    nome: nome.trim(),
    administradora_id: administradoraId,
    ...semIds,
  };
}

function useFormularioDoNovo(estado: Estado, onFechar: () => void) {
  const escopo = useEscopo() ?? null;
  const { executar, gravando } = useExecutar();
  const navegar = useNavigate();
  const { control, handleSubmit, reset } = useForm<Valores>({
    resolver: zodResolver(esquemaDoNovo(estado)),
    defaultValues: { nome: '', administradora: '' },
  });
  const nome = useWatch({ control, name: 'nome' });

  const criar = handleSubmit(async (valores) => {
    const novo = condominioNovo(valores, escopo);
    if (!(await executar((e) => criarCondominio(novo, e), 'Condomínio criado.'))) return;
    reset();
    onFechar();
    void navegar(`/condominios/${encodeURIComponent(novo.id)}`);
  });

  return { control, criar, escopo, desabilitado: gravando || nome.trim() === '' };
}

export function NovoCondominio({ aberto, estado, onFechar }: NovoProps): ReactNode {
  const { control, criar, escopo, desabilitado } = useFormularioDoNovo(estado, onFechar);

  return (
    <Dialog open={aberto} onClose={onFechar} fullWidth maxWidth="xs">
      <DialogTitle>Novo condomínio</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 2, pt: 1 }}>
        <CampoTexto control={control} name="nome" autoFocus label="Nome" sx={{ mt: 1 }} />
        {escopo === null && <CampoAdministradora control={control} estado={estado} />}
      </DialogContent>
      <AcoesDoNovo onFechar={onFechar} desabilitado={desabilitado} onCriar={() => void criar()} />
    </Dialog>
  );
}

function CampoAdministradora({ control, estado }: { control: Control<Valores>; estado: Estado }) {
  return (
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
  );
}

type AcoesProps = { onFechar: () => void; desabilitado: boolean; onCriar: () => void };

function AcoesDoNovo({ onFechar, desabilitado, onCriar }: AcoesProps): ReactNode {
  return (
    <DialogActions>
      <Button onClick={onFechar}>Cancelar</Button>
      <Button variant="contained" disabled={desabilitado} onClick={onCriar}>
        Criar condomínio
      </Button>
    </DialogActions>
  );
}
