import { TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { rotuloDoCampo } from '@/ferramentas/formularioDaFerramenta';
import { type ParDoPayload, TIPOS, tipoDe, tipoInferido } from './payloadLivre';

type Props = {
  par: ParDoPayload;
  travado: boolean;
  erroChave?: string | undefined;
  erroValor?: string | undefined;
  onMudar: (parcial: Partial<ParDoPayload>) => void;
};

export function CampoDoPar({ par, travado, erroChave, erroValor, onMudar }: Props): ReactNode {
  // Linha nova acompanha o que se digita até a pessoa escolher o tipo; linha gravada mantém o dela.
  const digitar = (valor: string) =>
    onMudar(par.fixo ? { valor } : { valor, tipo: tipoInferido(valor) });
  return (
    <>
      <TextField
        size="small"
        placeholder="limite"
        value={par.chave}
        disabled={travado}
        error={erroChave !== undefined}
        helperText={erroChave}
        slotProps={rotuloDoCampo('Chave')}
        onChange={(evento) => onMudar({ chave: evento.target.value })}
      />
      <TextField
        size="small"
        placeholder="3"
        value={par.valor}
        disabled={travado}
        error={erroValor !== undefined}
        helperText={erroValor}
        slotProps={rotuloDoCampo('Valor')}
        onChange={(evento) => digitar(evento.target.value)}
      />
      <SeletorDeTipo tipo={par.tipo} travado={travado} onEscolher={onMudar} />
    </>
  );
}

type SeletorProps = {
  tipo: ParDoPayload['tipo'];
  travado: boolean;
  onEscolher: (parcial: Partial<ParDoPayload>) => void;
};

function SeletorDeTipo({ tipo, travado, onEscolher }: SeletorProps): ReactNode {
  return (
    <TextField
      select
      size="small"
      value={tipo}
      disabled={travado}
      slotProps={{ select: { native: true }, htmlInput: { 'aria-label': 'Tipo do valor' } }}
      onChange={(evento) => onEscolher({ tipo: tipoDe(evento.target.value), fixo: true })}
    >
      {TIPOS.map((opcao) => (
        <option key={opcao} value={opcao}>
          {opcao}
        </option>
      ))}
    </TextField>
  );
}
