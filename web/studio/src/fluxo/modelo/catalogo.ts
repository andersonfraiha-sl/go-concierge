import type { SvgIconComponent } from '@mui/icons-material';
import AutoAwesomeOutlined from '@mui/icons-material/AutoAwesomeOutlined';
import ChatBubbleOutlineOutlined from '@mui/icons-material/ChatBubbleOutlineOutlined';
import CloudDownloadOutlined from '@mui/icons-material/CloudDownloadOutlined';
import DescriptionOutlined from '@mui/icons-material/DescriptionOutlined';
import HelpOutlineOutlined from '@mui/icons-material/HelpOutlineOutlined';
import ListAltOutlined from '@mui/icons-material/ListAltOutlined';
import ReplayOutlined from '@mui/icons-material/ReplayOutlined';
import SaveOutlined from '@mui/icons-material/SaveOutlined';
import TaskAltOutlined from '@mui/icons-material/TaskAltOutlined';
import { type Bloco, BlocoSchema, TIPOS_BLOCO, type TipoBloco } from './tipos';

type DescricaoDoBloco = { rotulo: string; descricao: string; cor: string; Icone: SvgIconComponent };

export const BLOCOS: Record<TipoBloco, DescricaoDoBloco> = {
  buscar_dados: {
    rotulo: 'Buscar dados',
    descricao: 'Consulta um sistema e guarda o resultado.',
    cor: '#00838F',
    Icone: CloudDownloadOutlined,
  },
  gravar_no_sistema: {
    rotulo: 'Gravar no sistema',
    descricao: 'Registra algo num sistema. Precisa de um Confirmar antes.',
    cor: '#EF6C00',
    Icone: SaveOutlined,
  },
  pedir_escolha: {
    rotulo: 'Pedir escolha',
    descricao: 'Mostra opções e guarda a escolhida.',
    cor: '#7B1FA2',
    Icone: ListAltOutlined,
  },
  perguntar: {
    rotulo: 'Perguntar',
    descricao: 'Pergunta ao morador o que falta.',
    cor: '#7B1FA2',
    Icone: HelpOutlineOutlined,
  },
  formulario: {
    rotulo: 'Formulário',
    descricao: 'Pede vários campos de uma vez.',
    cor: '#7B1FA2',
    Icone: DescriptionOutlined,
  },
  confirmar: {
    rotulo: 'Confirmar',
    descricao: 'Pede um sim antes de seguir.',
    cor: '#1034F2',
    Icone: TaskAltOutlined,
  },
  montar_texto: {
    rotulo: 'Montar texto',
    descricao: 'Escreve um texto com a IA.',
    cor: '#C2185B',
    Icone: AutoAwesomeOutlined,
  },
  reaproveitar: {
    rotulo: 'Reaproveitar resposta',
    descricao: 'Oferece usar a resposta dada antes na conversa.',
    cor: '#7B1FA2',
    Icone: ReplayOutlined,
  },
  responder: {
    rotulo: 'Responder',
    descricao: 'A resposta final ao morador.',
    cor: '#455A64',
    Icone: ChatBubbleOutlineOutlined,
  },
};

export const TIPOS_INSERIVEIS: readonly TipoBloco[] = TIPOS_BLOCO.filter(
  (tipo) => tipo !== 'responder',
);

/** Bloco vazio: o parse preenche os defaults de cada tipo. */
export function novoBloco(tipo: TipoBloco, id: string): Bloco {
  return BlocoSchema.parse({ id, tipo });
}

const SEM_CONFIGURAR = 'Clique para configurar';
const LIMITE_DO_RESUMO = 60;

export function resumoDoBloco(bloco: Bloco, tituloDaFerramenta: (nome: string) => string): string {
  return resumoBruto(bloco, tituloDaFerramenta) || SEM_CONFIGURAR;
}

function resumoBruto(bloco: Bloco, tituloDaFerramenta: (nome: string) => string): string {
  switch (bloco.tipo) {
    case 'buscar_dados':
    case 'gravar_no_sistema':
      return bloco.recurso && tituloDaFerramenta(bloco.recurso);
    case 'pedir_escolha':
      return bloco.pergunta;
    case 'perguntar':
    case 'reaproveitar':
      return bloco.campos.join(', ');
    case 'formulario':
      return bloco.de && `Campos de ${bloco.de}`;
    case 'confirmar':
    case 'montar_texto':
    case 'responder':
      return cortar(colapsar(bloco.texto));
  }
}

const colapsar = (texto: string): string => texto.replace(/\s+/g, ' ').trim();

const cortar = (texto: string): string =>
  texto.length > LIMITE_DO_RESUMO ? `${texto.slice(0, LIMITE_DO_RESUMO)}…` : texto;
