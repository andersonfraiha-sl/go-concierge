import { Box, Button, TextField } from '@mui/material';
import { useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import { removerCondominio, salvarCondominio, useEscopo, type Administradora, type Condominio } from './contratos';
import { SeletorDeAdministradora } from './SeletorDeAdministradora';
import { useExecutar } from './useExecutar';

type Props = { condominio: Condominio; administradoras: Administradora[] };

export function IdentificacaoSecao({ condominio, administradoras }: Props): ReactNode {
  const executar = useExecutar();
  const navegar = useNavigate();
  const doGestor = useEscopo() !== null;
  const [rascunho, setRascunho] = useState<Condominio>(condominio);
  const [removendo, setRemovendo] = useState(false);

  const campo = (rotulo: string, chave: 'nome' | 'account' | 'company_id' | 'tenant'): ReactNode => (
    <TextField
      label={rotulo}
      value={rascunho[chave]}
      onChange={(e) => setRascunho((atual) => ({ ...atual, [chave]: e.target.value }))}
    />
  );

  async function remover(): Promise<void> {
    setRemovendo(false);
    const removido = await executar((escopo) => removerCondominio(condominio.id, escopo), 'Condomínio removido.');
    if (removido) void navegar('/condominios');
  }

  return (
    <SectionCard titulo="Identificação" ajuda="Como os sistemas reconhecem este condomínio.">
      <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } }}>
        {campo('Nome', 'nome')}
        <SeletorDeAdministradora
          administradoras={administradoras}
          valor={rascunho.administradora_id}
          desabilitado={doGestor}
          onMudar={(id) => setRascunho((atual) => ({ ...atual, administradora_id: id }))}
        />
        {campo('Conta de integrações', 'account')}
        {campo('Empresa no Gruvi', 'company_id')}
        {campo('Canal do Atende', 'tenant')}
      </Box>
      <Box sx={{ display: 'flex', gap: 1, justifyContent: 'space-between' }}>
        <Button color="error" onClick={() => setRemovendo(true)}>
          Remover condomínio
        </Button>
        <Button
          variant="contained"
          disabled={rascunho.nome.trim() === ''}
          onClick={() => void executar((escopo) => salvarCondominio({ ...rascunho, nome: rascunho.nome.trim() }, escopo), 'Identificação salva.')}
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
