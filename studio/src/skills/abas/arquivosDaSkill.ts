import { useQueryClient } from '@tanstack/react-query';
import {
  caminhoDeDocumento,
  caminhoDeRegra,
  excluirArquivo,
  salvarArquivo,
  type Arquivo,
  type RecursoDeclarado,
} from '@/catalogo/contratos';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { paraIdentificador } from '../contratos';

type Agente = { arquivos: Record<string, Arquivo[]>; herdados: Record<string, Arquivo[]> };

const PASTA_REGRAS = 'rules';
const PASTA_DOCUMENTOS = 'knowledge';
const ARQUIVO_FORMATO_DA_RESPOSTA = 'output.md';
const EXTENSAO_DE_TEXTO = /\.(md|txt)$/i;
const ORIGEM_FLUXO = 'caso.yaml';

export const regrasDe = (agente: Agente): Arquivo[] => agente.arquivos[PASTA_REGRAS] ?? [];
export const documentosDe = (agente: Agente): Arquivo[] => agente.arquivos[PASTA_DOCUMENTOS] ?? [];
export const documentosHerdadosDe = (agente: Agente): Arquivo[] => agente.herdados[PASTA_DOCUMENTOS] ?? [];

export const ehTexto = (arquivo: Arquivo): boolean => EXTENSAO_DE_TEXTO.test(arquivo.caminho);
export const vemDoFluxo = (recurso: RecursoDeclarado): boolean => recurso.origem === ORIGEM_FLUXO;

const nomeDoArquivo = (caminho: string): string => caminho.split('/').pop() ?? caminho;

const comPrimeiraMaiuscula = (texto: string): string => texto.charAt(0).toUpperCase() + texto.slice(1);

export function tituloDoDocumento(caminho: string): string {
  const semExtensao = nomeDoArquivo(caminho).replace(EXTENSAO_DE_TEXTO, '');
  return comPrimeiraMaiuscula(semExtensao.replace(/[-_]+/g, ' '));
}

export function tituloDaRegra(caminho: string): string {
  return nomeDoArquivo(caminho) === ARQUIVO_FORMATO_DA_RESPOSTA ? 'Formato da resposta' : tituloDoDocumento(caminho);
}

/** Para arquivos que não são texto: mostra o nome como está. */
export const nomeParaExibir = (arquivo: Arquivo): string => (ehTexto(arquivo) ? tituloDoDocumento(arquivo.caminho) : nomeDoArquivo(arquivo.caminho));

export function formatarTamanho(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

type Tipo = 'regra' | 'documento';

const caminhoNovo: Record<Tipo, (nome: string) => string> = {
  regra: (nome) => caminhoDeRegra(`${paraIdentificador(nome)}.md`),
  documento: (nome) => caminhoDeDocumento(`${paraIdentificador(nome)}.md`),
};

const avisos: Record<Tipo, { criado: string; excluido: string; repetido: string }> = {
  regra: { criado: 'Regra criada.', excluido: 'Regra excluída.', repetido: 'Já existe uma regra com esse nome.' },
  documento: {
    criado: 'Documento criado.',
    excluido: 'Documento excluído.',
    repetido: 'Já existe um documento com esse nome.',
  },
};

const mensagemDe = (erro: unknown): string => (erro instanceof Error ? erro.message : String(erro));

/** Criar e excluir arquivos de texto da skill, refazendo a leitura do agente e avisando no rodapé. */
export function useGestaoDeArquivos(agente: string, tipo: Tipo, existentes: readonly Arquivo[]) {
  const cliente = useQueryClient();
  const { mostrar } = useSnackbar();
  const atualizar = () => cliente.invalidateQueries({ queryKey: ['agente', agente] });

  /** Rejeita com a mensagem para o campo de nome mostrar. */
  const criar = async (nome: string): Promise<void> => {
    const caminho = caminhoNovo[tipo](nome);
    if (existentes.some((existente) => existente.caminho === caminho)) throw new Error(avisos[tipo].repetido);
    await salvarArquivo({ agente, caminho, conteudo: '' });
    await atualizar();
    mostrar(avisos[tipo].criado);
  };

  const excluir = async (caminho: string): Promise<void> => {
    try {
      await excluirArquivo(agente, caminho);
    } catch (erro) {
      mostrar(mensagemDe(erro), 'error');
      return;
    }
    await atualizar();
    mostrar(avisos[tipo].excluido);
  };

  return { criar, excluir };
}
