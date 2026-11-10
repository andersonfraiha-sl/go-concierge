import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  TextField,
  Typography,
} from '@mui/material';
import { type ReactNode, useId, useState } from 'react';
import type { Bloco } from '@/fluxo/modelo/tipos';
import { BlocoPainel } from '@/fluxo/painel/BlocoPainel';
import { type Camada, type Diferenca, useCamada } from './contratos';
import { useCatalogoDaBase } from './FluxoDaCamada';
import { blocoDoConteudo } from './leituraDoTrecho';

type Props = {
  diferenca: Diferenca;
  camada: Camada;
  /** Onde a mistura começa: a mistura anterior ou, na primeira vez, a versão da pessoa. */
  inicial: string;
  /** Recebe o conteúdo canônico: texto puro, ou o JSON do bloco quando o trecho é uma etapa. */
  onConfirmar: (mistura: string) => void;
  onFechar: () => void;
};

/** Monte só quando aberto: o editor nasce de `inicial`. */
export function MisturaDialog({ diferenca, camada, inicial, onConfirmar, onFechar }: Props) {
  const [mistura, setMistura] = useState(inicial);
  const idDoTitulo = useId();
  return (
    <Dialog open fullWidth maxWidth="md" onClose={onFechar} aria-labelledby={idDoTitulo}>
      <DialogTitle id={idDoTitulo}>Editar a mistura · {diferenca.rotulo}</DialogTitle>
      <DialogContent sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
        <Typography variant="body2" color="textSecondary">
          Comece pela sua versão e leve para ela o que quiser da versão nova.
        </Typography>
        {diferenca.tipo === 'etapa' ? (
          <EditorDaEtapa camada={camada} mistura={mistura} onMudar={setMistura} />
        ) : (
          <EditorDeTexto rotulo={diferenca.rotulo} mistura={mistura} onMudar={setMistura} />
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onFechar}>Cancelar</Button>
        <Button variant="contained" onClick={() => onConfirmar(mistura)}>
          Usar esta mistura
        </Button>
      </DialogActions>
    </Dialog>
  );
}

type PropsDoTexto = { rotulo: string; mistura: string; onMudar: (mistura: string) => void };

function EditorDeTexto({ rotulo, mistura, onMudar }: PropsDoTexto): ReactNode {
  return (
    <TextField
      autoFocus
      fullWidth
      multiline
      minRows={4}
      label={`Mistura de ${rotulo}`}
      value={mistura}
      onChange={(evento) => onMudar(evento.target.value)}
    />
  );
}

type PropsDaEtapa = { camada: Camada; mistura: string; onMudar: (mistura: string) => void };

// As ferramentas e informações oferecidas são as da camada: a mistura nunca liga ferramenta de fora da base.
function EditorDaEtapa({ camada, mistura, onMudar }: PropsDaEtapa) {
  const lida = useCamada(camada.skill, camada.tipo, camada.id);
  const catalogo = useCatalogoDaBase(lida.data?.ferramentasDaBase ?? []);
  const bloco = blocoDoConteudo(mistura);
  if (bloco === null) return <EditorDeTexto rotulo="etapa" mistura={mistura} onMudar={onMudar} />;
  const mudar = (novo: Bloco) => onMudar(JSON.stringify(novo));
  return (
    <BlocoPainel
      bloco={bloco}
      problemas={[]}
      catalogo={catalogo}
      informacoes={lida.data?.documento.fluxo.informacoes ?? []}
      editavel
      permiteExcluir={false}
      onMudar={mudar}
      onExcluir={() => undefined}
    />
  );
}
