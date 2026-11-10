import ExpandMoreIcon from '@mui/icons-material/ExpandMore';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  DialogActions,
  DialogContent,
  TextField,
  Typography,
} from '@mui/material';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { type ReactNode, useState } from 'react';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import {
  CHAVE_DAS_ATUALIZACOES,
  chaveDaPreviaDaBase,
  type PreviaDaBase,
  publicarBase,
} from './contratos';
import { ErroDaPublicacao } from './ErroDaPublicacao';
import type { RotulosDasEtapas } from './leituraDoTrecho';
import { classificarErroDaPublicacao, exigeMotivo } from './regrasDaPublicacao';
import { TrechoParaPublicar } from './TrechoParaPublicar';

type Props = { skill: string; previa: PreviaDaBase; onFechar: () => void };

function rotulosDasEtapas({ diferencas }: PreviaDaBase): RotulosDasEtapas {
  return new Map(diferencas.filter((d) => d.tipo === 'etapa').map((d) => [d.trecho, d.rotulo]));
}

const descreverCamadas = (camadas: number): string =>
  camadas === 1 ? '1 camada' : `${camadas} camadas`;

function useEnvioDaBase(skill: string, onFechar: () => void) {
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();
  return useMutation({
    mutationFn: (corpo: Parameters<typeof publicarBase>[1]) => publicarBase(skill, corpo),
    onSuccess: async ({ numero, camadas }) => {
      await Promise.all(
        [chaveDaPreviaDaBase(skill), CHAVE_DAS_ATUALIZACOES].map((queryKey) =>
          cliente.invalidateQueries({ queryKey }),
        ),
      );
      mostrar(`Versão ${numero} publicada para ${descreverCamadas(camadas)}.`);
      onFechar();
    },
  });
}

export function FormularioDePublicacao({ skill, previa, onFechar }: Props): ReactNode {
  const [obrigatorios, setObrigatorios] = useState<string[]>([]);
  const [resumo, setResumo] = useState('');
  const [motivo, setMotivo] = useState('');
  const [encerraEm, setEncerraEm] = useState('');
  const envio = useEnvioDaBase(skill, onFechar);

  const semMudanca = previa.numero !== null && previa.diferencas.length === 0;
  const motivoPendente = exigeMotivo(obrigatorios, encerraEm) && motivo.trim() === '';
  // Sem mudança só há o que publicar se a skill for encerrada.
  const podePublicar = (!semMudanca || encerraEm !== '') && !motivoPendente && !envio.isPending;
  const rotulos = rotulosDasEtapas(previa);

  const alternar = (trecho: string, marcado: boolean) =>
    setObrigatorios((atuais) =>
      marcado ? [...atuais, trecho] : atuais.filter((atual) => atual !== trecho),
    );

  return (
    <>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        {semMudanca ? (
          <Alert severity="info">Nada mudou desde a versão {previa.numero}.</Alert>
        ) : (
          <Typography color="textSecondary">
            {previa.numero === null
              ? 'Primeira versão publicada da skill.'
              : `Mudanças desde a versão ${previa.numero}.`}{' '}
            Um trecho obrigatório entra em todas as camadas, mesmo onde alguém o alterou.
          </Typography>
        )}
        {previa.diferencas.map((diferenca) => (
          <TrechoParaPublicar
            key={diferenca.trecho}
            diferenca={diferenca}
            rotulosDasEtapas={rotulos}
            obrigatorio={obrigatorios.includes(diferenca.trecho)}
            onMudarObrigatorio={(marcado) => alternar(diferenca.trecho, marcado)}
          />
        ))}
        <TextField
          label="Resumo da versão"
          helperText="Aparece para quem recebe a atualização."
          value={resumo}
          onChange={(evento) => setResumo(evento.target.value)}
          multiline
          minRows={2}
          fullWidth
        />
        <TextField
          label="Motivo"
          helperText={
            motivoPendente
              ? 'Escreva o motivo: ele aparece para quem tinha alterado o trecho.'
              : 'Obrigatório para trecho obrigatório ou encerramento.'
          }
          error={motivoPendente}
          value={motivo}
          onChange={(evento) => setMotivo(evento.target.value)}
          multiline
          minRows={2}
          fullWidth
        />
        <Encerramento valor={encerraEm} onMudar={setEncerraEm} />
        {envio.isError && <ErroDaPublicacao erro={classificarErroDaPublicacao(envio.error)} />}
      </DialogContent>
      <DialogActions>
        <Button onClick={onFechar}>Cancelar</Button>
        <Button
          variant="contained"
          disabled={!podePublicar}
          onClick={() =>
            envio.mutate({
              resumo: resumo.trim(),
              obrigatorios,
              motivo: motivo.trim(),
              encerraEm: encerraEm === '' ? null : encerraEm,
            })
          }
        >
          Publicar
        </Button>
      </DialogActions>
    </>
  );
}

function Encerramento({
  valor,
  onMudar,
}: {
  valor: string;
  onMudar: (valor: string) => void;
}): ReactNode {
  return (
    <Accordion disableGutters variant="outlined">
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>Encerrar esta skill</AccordionSummary>
      <AccordionDetails>
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
          <Typography variant="body2" color="textSecondary">
            Depois desta data a skill deixa de ser oferecida em todas as camadas.
          </Typography>
          <TextField
            type="date"
            label="Encerra em"
            value={valor}
            onChange={(evento) => onMudar(evento.target.value)}
            slotProps={{ inputLabel: { shrink: true } }}
            sx={{ maxWidth: 220 }}
          />
        </Box>
      </AccordionDetails>
    </Accordion>
  );
}
