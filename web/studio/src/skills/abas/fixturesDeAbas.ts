import type { Eu } from '@/acesso/eu';

export const AGENTE = 'skills/agendar-mudanca';

export const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos', 'ferramentas.editar'],
  administradoraId: null,
  dev: false,
};
export const gestor: Eu = { ...construtor, papel: 'gestor', administradoraId: 'adm-1' };

export const arquivo = (caminho: string, bytes = 10) => ({ caminho, bytes });

type Conteudo = {
  regras?: ReturnType<typeof arquivo>[];
  documentos?: ReturnType<typeof arquivo>[];
  herdados?: ReturnType<typeof arquivo>[];
  recursos?: unknown[];
};

/** Resposta de `GET /api/agente` no formato que o Go devolve. */
export const agenteLido = ({
  regras = [],
  documentos = [],
  herdados = [],
  recursos = [],
}: Conteudo = {}) => ({
  agente: {
    caminho: AGENTE,
    nome: 'agendar-mudanca',
    descricao: '',
    persona: '',
    arquivos: { rules: regras, knowledge: documentos },
    herdados: { knowledge: herdados },
    problemas: [],
  },
  recursos,
});

export const ferramenta = (name: string, write: boolean, titulo = name) => ({
  name,
  titulo,
  kind: 'api',
  visibility: 'private',
  write,
  description: `Descrição de ${titulo}`,
});

export const recursoDeclarado = (
  name: string,
  origem: string,
  extras: { ok?: boolean; problema?: string } = {},
) => ({
  nome: name,
  origem,
  ok: extras.ok ?? true,
  conferido: true,
  ...(extras.problema === undefined ? {} : { problema: extras.problema }),
  recurso: ferramenta(name, false, name),
});
