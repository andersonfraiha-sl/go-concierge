import { Box, ToggleButton, ToggleButtonGroup, Typography } from '@mui/material';
import { type MouseEvent, type ReactNode, useState } from 'react';
import { ConteudoDoTrecho } from './ConteudoDoTrecho';
import type { Camada, Diferenca } from './contratos';
import type { Escolha } from './decisoes';
import { lerConteudo, type RotulosDasEtapas } from './leituraDoTrecho';
import { MisturaDialog } from './MisturaDialog';

type Props = {
  diferenca: Diferenca;
  camada: Camada;
  escolha: Escolha | undefined;
  rotulosDasEtapas: RotulosDasEtapas;
  editavel: boolean;
  onEscolher: (escolha: Escolha) => void;
};

const TIPOS_QUE_ACEITAM_MISTURA: readonly Diferenca['tipo'][] = ['texto', 'orientacao', 'etapa'];

const ehConflito = (d: Diferenca): boolean => d.situacao === 'conflito';

// Só dá para misturar quando há texto novo para levar à versão da pessoa.
const aceitaMistura = (d: Diferenca): boolean =>
  ehConflito(d) && d.depois !== null && TIPOS_QUE_ACEITAM_MISTURA.includes(d.tipo);

function rotuloDeManter(d: Diferenca): string {
  if (!ehConflito(d)) return 'Manter como está';
  return d.minha === null ? 'Manter removido' : 'Ficar com a minha';
}

function rotuloDeAceitar(d: Diferenca): string {
  if (d.depois === null) return 'Aceitar a remoção';
  return ehConflito(d) ? 'Aceitar a nova' : 'Aceitar a mudança';
}

export function EscolhaDoTrecho({
  diferenca,
  camada,
  escolha,
  rotulosDasEtapas,
  editavel,
  onEscolher,
}: Props): ReactNode {
  const [misturando, setMisturando] = useState(false);
  const travada = diferenca.obrigatorio;
  const atual = escolha?.escolha ?? null;

  const escolherSimples = (_: MouseEvent<HTMLElement>, valor: 'aceitar' | 'manter' | null) => {
    if (valor !== null) onEscolher({ escolha: valor });
  };
  const abrirMistura = (evento: MouseEvent<HTMLElement>) => {
    // Sem isso o grupo marcaria "mistura" antes de existir uma.
    evento.preventDefault();
    setMisturando(true);
  };

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      <ToggleButtonGroup
        exclusive
        size="small"
        value={atual}
        onChange={escolherSimples}
        disabled={travada || !editavel}
        aria-label={`Sua escolha para ${diferenca.rotulo}`}
      >
        <ToggleButton value="manter">{rotuloDeManter(diferenca)}</ToggleButton>
        <ToggleButton value="aceitar">{rotuloDeAceitar(diferenca)}</ToggleButton>
        {aceitaMistura(diferenca) && (
          <ToggleButton value="misturar" onClick={abrirMistura}>
            Editar a mistura
          </ToggleButton>
        )}
      </ToggleButtonGroup>
      {travada && (
        <Typography variant="caption" color="textSecondary">
          Obrigatório: a Superlógica exige esta troca, então ela entra de qualquer jeito.
        </Typography>
      )}
      {escolha?.escolha === 'misturar' && (
        <Box aria-label="Sua mistura">
          <Typography variant="caption" color="textSecondary">
            Sua mistura
          </Typography>
          <ConteudoDoTrecho
            conteudo={lerConteudo(diferenca.tipo, escolha.mistura, rotulosDasEtapas)}
          />
        </Box>
      )}
      {misturando && (
        <MisturaDialog
          diferenca={diferenca}
          camada={camada}
          inicial={
            escolha?.escolha === 'misturar'
              ? escolha.mistura
              : (diferenca.minha ?? diferenca.depois ?? '')
          }
          onFechar={() => setMisturando(false)}
          onConfirmar={(mistura) => {
            setMisturando(false);
            onEscolher({ escolha: 'misturar', mistura });
          }}
        />
      )}
    </Box>
  );
}
