import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import type { Bloco, Informacao, TipoBloco } from '@/fluxo/modelo/tipos';
import { CamposDeFerramenta } from './CamposDeFerramenta';
import { CamposDePedirEscolha } from './CamposDePedirEscolha';
import { SelecaoDeInformacoes } from './SelecaoDeInformacoes';
import { Texto } from './Texto';

type BlocoDo<T extends TipoBloco> = Extract<Bloco, { tipo: T }>;

/** O que se sabe do catálogo de ferramentas; enquanto não está pronto, nenhuma é "não encontrada". */
export type CatalogoDeFerramentas =
  | { estado: 'carregando' }
  | { estado: 'erro' }
  | { estado: 'pronto'; ferramentas: readonly Ferramenta[] };

type Contexto = {
  editavel: boolean;
  catalogo: CatalogoDeFerramentas;
  informacoes: readonly Informacao[];
  /** Recebe `Bloco`, não o tipo exato: escolher a ferramenta pode trocar Buscar por Gravar. */
  onMudar: (bloco: Bloco) => void;
};

export type CamposProps<T extends TipoBloco> = Contexto & { bloco: BlocoDo<T> };

const CAMPOS: { [T in TipoBloco]: (props: CamposProps<T>) => ReactNode } = {
  buscar_dados: CamposDeFerramenta,
  gravar_no_sistema: CamposDeFerramenta,
  pedir_escolha: CamposDePedirEscolha,
  perguntar: ({ bloco, editavel, informacoes, onMudar }) => (
    <SelecaoDeInformacoes
      rotulo="Informações perguntadas"
      valor={bloco.campos}
      informacoes={informacoes}
      editavel={editavel}
      onMudar={(campos) => onMudar({ ...bloco, campos })}
    />
  ),
  formulario: ({ bloco, editavel, onMudar }) => (
    <>
      <Texto
        rotulo="Campos vêm de"
        valor={bloco.de}
        editavel={editavel}
        onMudar={(de) => onMudar({ ...bloco, de })}
      />
      <Texto
        rotulo="Guardar respostas em"
        valor={bloco.guardarEm}
        editavel={editavel}
        onMudar={(guardarEm) => onMudar({ ...bloco, guardarEm })}
      />
    </>
  ),
  confirmar: ({ bloco, editavel, onMudar }) => (
    <Texto
      rotulo="Pergunta de confirmação"
      valor={bloco.texto}
      editavel={editavel}
      linhas={3}
      onMudar={(texto) => onMudar({ ...bloco, texto })}
    />
  ),
  montar_texto: ({ bloco, editavel, onMudar }) => (
    <Texto
      rotulo="Instrução para a IA"
      valor={bloco.texto}
      editavel={editavel}
      linhas={5}
      onMudar={(texto) => onMudar({ ...bloco, texto })}
    />
  ),
  reaproveitar: ({ bloco, editavel, informacoes, onMudar }) => (
    <>
      <SelecaoDeInformacoes
        rotulo="Informações reaproveitadas"
        valor={bloco.campos}
        informacoes={informacoes}
        editavel={editavel}
        onMudar={(campos) => onMudar({ ...bloco, campos })}
      />
      <Texto
        rotulo="Guardar a resposta em"
        valor={bloco.confirmacao}
        editavel={editavel}
        onMudar={(confirmacao) => onMudar({ ...bloco, confirmacao })}
      />
      <Texto
        rotulo="Pergunta"
        valor={bloco.pergunta}
        editavel={editavel}
        onMudar={(pergunta) => onMudar({ ...bloco, pergunta })}
      />
    </>
  ),
  responder: ({ bloco, editavel, onMudar }) => (
    <Texto
      rotulo="Resposta"
      valor={bloco.texto}
      editavel={editavel}
      linhas={5}
      onMudar={(texto) => onMudar({ ...bloco, texto })}
    />
  ),
};

/** Campos do bloco conforme o tipo. O que o Studio não edita (`extra` e afins) volta intacto pelo spread. */
export function CamposDoBloco(props: CamposProps<TipoBloco>): ReactNode {
  return renderizar(props.bloco.tipo, props);
}

function renderizar<T extends TipoBloco>(tipo: T, props: CamposProps<T>): ReactNode {
  const Campos: (props: CamposProps<T>) => ReactNode = CAMPOS[tipo];
  return <Campos {...props} />;
}
