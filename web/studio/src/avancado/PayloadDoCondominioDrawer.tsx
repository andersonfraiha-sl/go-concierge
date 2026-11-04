import { zodResolver } from '@hookform/resolvers/zod';
import { Box, Button, Drawer, Typography } from '@mui/material';
import { type ReactNode, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { z } from 'zod';
import type { Ferramenta } from '@/catalogo/contratos';
import type { Valor } from '@/condominios/contratos';
import { tituloDe } from '@/ferramentas/contratos';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import { ConfirmDialog } from '@/shared/ui/ConfirmDialog';
import { CampoDoPar } from './CampoDoPar';
import { type ParDoPayload, payloadEmPares } from './payloadLivre';

const EsquemaDoFormulario = z.object({
  pares: z.array(z.object({ chave: z.string(), valor: z.string() })),
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
  const { control, handleSubmit } = useForm({
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
        <Controller
          control={control}
          name="pares"
          render={({ field }) => (
            <ListaEditavel<ParDoPayload>
              titulo="Valores"
              adicionar="Adicionar valor"
              linhas={field.value}
              desabilitado={travado}
              criarLinha={() => ({ chave: '', valor: '' })}
              onMudar={field.onChange}
              renderLinha={(par, mudar) => (
                <CampoDoPar par={par} travado={travado} onMudar={mudar} />
              )}
            />
          )}
        />
        <Box sx={{ display: 'flex', gap: 1, justifyContent: 'flex-end' }}>
          <Button
            color="error"
            disabled={travado}
            onClick={() => setConfirmando(true)}
            sx={{ mr: 'auto' }}
          >
            Limpar deste condomínio
          </Button>
          <Button onClick={onFechar}>Fechar</Button>
          <Button type="submit" variant="contained" disabled={travado}>
            Salvar
          </Button>
        </Box>
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
