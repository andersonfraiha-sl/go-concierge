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

type CategoriaDeBloco = 'start' | 'send' | 'ask' | 'logic' | 'integrate' | 'ai' | 'end';

type DescricaoDoBloco = {
  rotulo: string;
  descricao: string;
  categoria: CategoriaDeBloco;
  Icone: SvgIconComponent;
};

export const BLOCOS: Record<TipoBloco, DescricaoDoBloco> = {
  buscar_dados: {
    rotulo: 'Buscar dados',
    descricao: 'Consulta um sistema e guarda o resultado.',
    categoria: 'integrate',
    Icone: CloudDownloadOutlined,
  },
  gravar_no_sistema: {
    rotulo: 'Gravar no sistema',
    descricao: 'Registra algo num sistema. Precisa de um Confirmar antes.',
    categoria: 'logic',
    Icone: SaveOutlined,
  },
  pedir_escolha: {
    rotulo: 'Pedir escolha',
    descricao: 'Mostra opções e guarda a escolhida.',
    categoria: 'ask',
    Icone: ListAltOutlined,
  },
  perguntar: {
    rotulo: 'Perguntar',
    descricao: 'Pergunta ao morador o que falta.',
    categoria: 'ask',
    Icone: HelpOutlineOutlined,
  },
  formulario: {
    rotulo: 'Formulário',
    descricao: 'Pede vários campos de uma vez.',
    categoria: 'ask',
    Icone: DescriptionOutlined,
  },
  confirmar: {
    rotulo: 'Confirmar',
    descricao: 'Pede um sim antes de seguir.',
    categoria: 'send',
    Icone: TaskAltOutlined,
  },
  montar_texto: {
    rotulo: 'Montar texto',
    descricao: 'Escreve um texto com a IA.',
    categoria: 'ai',
    Icone: AutoAwesomeOutlined,
  },
  reaproveitar: {
    rotulo: 'Reaproveitar resposta',
    descricao: 'Oferece usar a resposta dada antes na conversa.',
    categoria: 'ask',
    Icone: ReplayOutlined,
  },
  responder: {
    rotulo: 'Responder',
    descricao: 'A resposta final ao morador.',
    categoria: 'end',
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

const identidade = (nome: string): string => nome;

/** "a" -> "a"; "a, b" -> "a e b"; "a, b, c" -> "a, b e c". */
const juntarComE = (nomes: readonly string[]): string =>
  nomes.length <= 1 ? (nomes[0] ?? '') : `${nomes.slice(0, -1).join(', ')} e ${nomes.at(-1)}`;

export function resumoDoBloco(
  bloco: Bloco,
  tituloDaFerramenta: (nome: string) => string,
  /** Nome amigável de uma informação pedida (campo do fluxo); sem lookup, cai no próprio nome. */
  tituloDaInformacao: (nome: string) => string = identidade,
): string {
  return resumoBruto(bloco, tituloDaFerramenta, tituloDaInformacao) || SEM_CONFIGURAR;
}

function resumoBruto(
  bloco: Bloco,
  tituloDaFerramenta: (nome: string) => string,
  tituloDaInformacao: (nome: string) => string,
): string {
  switch (bloco.tipo) {
    case 'buscar_dados':
    case 'gravar_no_sistema':
      return bloco.recurso && tituloDaFerramenta(bloco.recurso);
    case 'pedir_escolha':
      return bloco.pergunta;
    case 'perguntar':
    case 'reaproveitar':
      return juntarComE(bloco.campos.map(tituloDaInformacao));
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
