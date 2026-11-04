import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button, Drawer, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { type Control, Controller, type FieldErrors, useForm } from 'react-hook-form';
import { z } from 'zod';
import type { Ferramenta } from '@/catalogo/contratos';
import type { Valor } from '@/condominios/contratos';
import { tituloDe } from '@/ferramentas/contratos';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { CampoDoPar } from './CampoDoPar';
import { errosDosPares, type ParDoPayload, parNovo, payloadEmPares, TIPOS } from './payloadLivre';

const EsquemaDoFormulario = z.object({
  pares: z
    .array(
      z.object({
        chave: z.string(),
        valor: z.string(),
        tipo: z.enum(TIPOS),
        fixo: z.boolean(),
      }),
    )
    .superRefine((pares, contexto) => {
      for (const { indice, campo, mensagem } of errosDosPares(pares)) {
        contexto.addIssue({ code: 'custom', message: mensagem, path: [indice, campo] });
      }
    }),
});

type Props = {
  recurso: Ferramenta;
  nomeDoCondominio: string;
  payload: Record<string, Valor>;
  podeEditar: boolean;
  gravando: boolean;
  onSalvar: (pares: readonly ParDoPayload[]) => void;
  onLimpar: () => void;
  onFechar: () => void;
};

type Formulario = z.infer<typeof EsquemaDoFormulario>;

export function PayloadDoCondominioDrawer({
  recurso,
  nomeDoCondominio,
  payload,
  podeEditar,
  gravando,
  onSalvar,
  onLimpar,
  onFechar,
}: Props): ReactNode {
  const [confirmando, setConfirmando] = useState(false);
  const { control, handleSubmit, formState } = useForm({
    resolver: zodResolver(EsquemaDoFormulario),
    defaultValues: { pares: payloadEmPares(payload) },
  });
  const travado = !podeEditar || gravando;
  const titulo = tituloDe(recurso);

  return (
    <Drawer
      anchor="right"
      open
      onClose={onFechar}
      slotProps={{
        paper: {
          sx: { width: 'min(420px, 100vw)' },
          role: 'dialog',
          'aria-label': `${titulo} · ${nomeDoCondominio}`,
        },
      }}
    >
      <Box
        component="form"
        noValidate
        onSubmit={(evento) => void handleSubmit(({ pares }) => onSalvar(pares))(evento)}
        sx={{ p: 2, display: 'flex', flexDirection: 'column', gap: 2 }}
      >
        <Typography variant="h6" component="h2">
          {titulo}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          Valores só de {nomeDoCondominio}. Número e true/false são gravados como tal.
        </Typography>
        <ListaDePares control={control} erros={formState.errors.pares} travado={travado} />
        <RodapeDoDrawer
          travado={travado}
          podeLimpar={Object.keys(payload).length > 0}
          onLimpar={() => setConfirmando(true)}
          onFechar={onFechar}
        />
      </Box>
      <ConfirmDialog
        aberto={confirmando}
        titulo="Limpar ajustes técnicos"
        texto={`Apagar os valores próprios de ${nomeDoCondominio} em "${titulo}"? Passa a valer o Padrão.`}
        confirmar="Limpar"
        onCancelar={() => setConfirmando(false)}
        onConfirmar={() => {
          setConfirmando(false);
          onLimpar();
        }}
      />
    </Drawer>
  );
}

type ListaProps = {
  control: Control<Formulario>;
  erros: FieldErrors<Formulario>['pares'];
  travado: boolean;
};

function ListaDePares({ control, erros, travado }: ListaProps): ReactNode {
  return (
    <Controller
      control={control}
      name="pares"
      render={({ field }) => (
        <ListaEditavel<ParDoPayload>
          titulo="Valores"
          adicionar="Adicionar valor"
          linhas={field.value}
          desabilitado={travado}
          criarLinha={parNovo}
          onMudar={field.onChange}
          renderLinha={(par, mudar) => {
            const errosDaLinha = erros?.[field.value.indexOf(par)];
            return (
              <CampoDoPar
                par={par}
                travado={travado}
                erroChave={errosDaLinha?.chave?.message}
                erroValor={errosDaLinha?.valor?.message}
                onMudar={mudar}
              />
            );
          }}
        />
      )}
    />
  );
}

type RodapeProps = {
  travado: boolean;
  podeLimpar: boolean;
  onLimpar: () => void;
  onFechar: () => void;
};

function RodapeDoDrawer({ travado, podeLimpar, onLimpar, onFechar }: RodapeProps): ReactNode {
  return (
    <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
      <Button
        color="error"
        disabled={travado || !podeLimpar}
        onClick={onLimpar}
        sx={{ mr: 'auto' }}
      >
        Limpar deste condomínio
      </Button>
      <Button onClick={onFechar}>Fechar</Button>
      <Button type="submit" variant="contained" disabled={travado}>
        Salvar
      </Button>
    </Box>
  );
}
