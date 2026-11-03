import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { useNavigate } from 'react-router';
import { z } from 'zod';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  type Administradora,
  type Condominio,
  removerCondominio,
  salvarCondominio,
  useEscopo,
} from './contratos';
import { SeletorDeAdministradora } from './SeletorDeAdministradora';
import { useExecutar } from './useExecutar';

type Props = { condominio: Condominio; administradoras: Administradora[] };

const IdentificacaoSchema = z.object({
  id: z.string(),
  nome: z.string().refine((nome) => nome.trim() !== '', 'Informe o nome.'),
  administradora_id: z.string(),
  account: z.string(),
  company_id: z.string(),
  tenant: z.string(),
});

export function IdentificacaoSecao({ condominio, administradoras }: Props): ReactNode {
  const { executar, gravando } = useExecutar();
  const navegar = useNavigate();
  const doGestor = useEscopo() !== null;
  const [removendo, setRemovendo] = useState(false);
  const { control, handleSubmit } = useForm({
    resolver: zodResolver(IdentificacaoSchema),
    defaultValues: condominio,
  });
  const nome = useWatch({ control, name: 'nome' });

  const salvar = handleSubmit((rascunho) =>
    executar(
      (escopo) => salvarCondominio({ ...rascunho, nome: rascunho.nome.trim() }, escopo),
      'Identificação salva.',
    ),
  );

  const campo = (rotulo: string, chave: 'nome' | 'account' | 'company_id' | 'tenant') => (
    <CampoTexto control={control} name={chave} label={rotulo} />
  );

  async function remover(): Promise<void> {
    setRemovendo(false);
    const removido = await executar(
      (escopo) => removerCondominio(condominio.id, escopo),
      'Condomínio removido.',
    );
    if (removido) void navegar('/condominios');
  }

  return (
    <SectionCard titulo="Identificação" ajuda="Como os sistemas reconhecem este condomínio.">
      <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } }}>
        {campo('Nome', 'nome')}
        <Controller
          control={control}
          name="administradora_id"
          render={({ field }) => (
            <SeletorDeAdministradora
              administradoras={administradoras}
              valor={field.value}
              desabilitado={doGestor}
              onMudar={field.onChange}
            />
          )}
        />
        {campo('Conta de integrações', 'account')}
        {campo('Empresa no Gruvi', 'company_id')}
        {campo('Canal do Atende', 'tenant')}
      </Box>
      <Box sx={{ display: 'flex', gap: 1, justifyContent: 'space-between' }}>
        <Button color="error" disabled={gravando} onClick={() => setRemovendo(true)}>
          Remover condomínio
        </Button>
        <Button
          variant="contained"
          disabled={gravando || nome.trim() === ''}
          onClick={() => void salvar()}
        >
          Salvar
        </Button>
      </Box>
      <ConfirmDialog
        aberto={removendo}
        titulo="Remover condomínio"
        texto={`Remover "${condominio.nome}"? As decisões dele somem junto.`}
        confirmar="Remover"
        onConfirmar={() => void remover()}
        onCancelar={() => setRemovendo(false)}
      />
    </SectionCard>
  );
}
