import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button, Chip, TextField } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useNavigate } from 'react-router';
import { z } from 'zod';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { useValoresDoFormulario } from '@/shared/autosave/useValoresDoFormulario';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { SectionCard } from '@/shared/ui/SectionCard';
import {
  type Administradora,
  type Condominio,
  faltamIds,
  removerCondominio,
  salvarCondominio,
  useAplicarMapeamento,
  useEscopo,
} from './contratos';
import { type CampoDeIdentificacao, validarIdentificacao } from './identificacao';
import { SeletorDeAdministradora } from './SeletorDeAdministradora';
import { useExecutar } from './useExecutar';

type Props = { condominio: Condominio; administradoras: Administradora[] };

const BLOQUEIO_POR_ERRO = 'corrija o campo marcado.';

const IdentificacaoSchema = z
  .object({
    id: z.string(),
    nome: z.string(),
    administradora_id: z.string(),
    account: z.string(),
    company_id: z.string(),
    tenant: z.string(),
  })
  .superRefine((valores, ctx) => {
    const erros = validarIdentificacao(valores);
    for (const campo of Object.keys(erros) as CampoDeIdentificacao[]) {
      ctx.addIssue({ code: z.ZodIssueCode.custom, path: [campo], message: erros[campo] });
    }
  });

export function IdentificacaoSecao({ condominio, administradoras }: Props): ReactNode {
  const aplicar = useAplicarMapeamento();
  const escopoBruto = useEscopo();
  const escopo = escopoBruto ?? null;
  const doGestor = escopoBruto !== null;
  const { executar, gravando } = useExecutar();
  const navegar = useNavigate();
  const [removendo, setRemovendo] = useState(false);
  const { control } = useForm({
    resolver: zodResolver(IdentificacaoSchema),
    defaultValues: condominio,
    mode: 'onChange',
  });
  const valores = useValoresDoFormulario(control);
  const temErro = Object.keys(validarIdentificacao(valores)).length > 0;

  const { estado: salvamento, tentarDeNovo } = useAutosave({
    valor: valores,
    bloqueio: temErro ? BLOQUEIO_POR_ERRO : null,
    salvar: async (rascunho) => {
      aplicar(await salvarCondominio({ ...rascunho, nome: rascunho.nome.trim() }, escopo));
    },
  });

  const campo = (
    rotulo: string,
    chave: 'nome' | 'account' | 'company_id' | 'tenant',
    opcoes: { placeholder?: string; required?: boolean } = {},
  ) => <CampoTexto control={control} name={chave} label={rotulo} {...opcoes} />;

  async function remover(): Promise<void> {
    setRemovendo(false);
    const removido = await executar(
      (escopoDaRemocao) => removerCondominio(condominio.id, escopoDaRemocao),
      'Condomínio removido.',
    );
    if (removido) void navegar('/condominios');
  }

  return (
    <SectionCard
      titulo="Identificação"
      ajuda="Como os sistemas reconhecem este condomínio."
      acao={
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          {faltamIds(valores) && (
            <Chip size="small" color="warning" label="Faltam ids de integração" />
          )}
          <SaveIndicator estado={salvamento} onTentarDeNovo={() => void tentarDeNovo()} />
        </Box>
      }
    >
      <Box sx={{ display: 'grid', gap: 2, gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' } }}>
        <TextField label="Id" value={condominio.id} disabled fullWidth />
        {campo('Nome', 'nome', { required: true })}
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
        {campo('Conta de integrações', 'account', { placeholder: 'cond-001' })}
        {campo('Empresa no Gruvi', 'company_id', { placeholder: '5982' })}
        {campo('Canal do Atende', 'tenant', { placeholder: '69a5a131e67bd8e2f54e5981' })}
      </Box>
      <Box sx={{ display: 'flex', justifyContent: 'flex-end' }}>
        <Button color="error" disabled={gravando} onClick={() => setRemovendo(true)}>
          Remover condomínio
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
