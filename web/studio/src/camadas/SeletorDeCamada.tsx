import { MenuItem, TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { useNavigate } from 'react-router';
import { useEu } from '@/acesso/eu';
import { useEstado } from '@/condominios/contratos';
import { rotaDaCamada } from './camadaDaRota';
import type { Camada } from './contratos';

export type OpcaoDeCamada = { rota: string; rotulo: string; camada: Camada | null };

const ROTULO_DA_BASE = 'Superlógica';
const ROTULO_DA_ADMINISTRADORA = 'Sua administradora';

const rotaDaBase = (skill: string): string => `/skills/${encodeURIComponent(skill)}`;

/** Camadas que o usuário pode abrir para a skill; `camada: null` é a base (o editor de hoje). */
export function useOpcoesDeCamada(skill: string): OpcaoDeCamada[] {
  const { data: eu } = useEu();
  const { data: estado } = useEstado();
  const administradora = eu?.administradoraId ?? null;
  const camadaDaAdministradora: Camada[] =
    administradora === null ? [] : [{ skill, tipo: 'administradora', id: administradora }];
  const condominios: Camada[] = (estado?.mapeamento.condominios ?? []).map((condominio) => ({
    skill,
    tipo: 'condominio',
    id: condominio.id,
  }));
  const nomes = new Map(
    (estado?.mapeamento.condominios ?? []).map((condominio) => [condominio.id, condominio.nome]),
  );

  return [
    { rota: rotaDaBase(skill), rotulo: ROTULO_DA_BASE, camada: null },
    ...[...camadaDaAdministradora, ...condominios].map((camada) => ({
      rota: rotaDaCamada(camada),
      rotulo:
        camada.tipo === 'administradora'
          ? ROTULO_DA_ADMINISTRADORA
          : (nomes.get(camada.id) ?? camada.id),
      camada,
    })),
  ];
}

type Props = {
  skill: string;
  /** Camada aberta agora; ausente é a base. */
  atual?: Camada;
};

export function SeletorDeCamada({ skill, atual }: Props): ReactNode {
  const navegar = useNavigate();
  const opcoes = useOpcoesDeCamada(skill);
  const rotaPedida = atual ? rotaDaCamada(atual) : rotaDaBase(skill);
  // Enquanto a lista de condomínios carrega, a camada aberta ainda não é opção: valor vazio evita aviso do MUI.
  const rotaAtual = opcoes.some((opcao) => opcao.rota === rotaPedida) ? rotaPedida : '';

  return (
    <TextField
      select
      size="small"
      label="Versão"
      value={rotaAtual}
      onChange={(evento) => void navegar(evento.target.value)}
      sx={{ minWidth: 200 }}
    >
      {opcoes.map((opcao) => (
        <MenuItem key={opcao.rota} value={opcao.rota}>
          {opcao.rotulo}
        </MenuItem>
      ))}
    </TextField>
  );
}
