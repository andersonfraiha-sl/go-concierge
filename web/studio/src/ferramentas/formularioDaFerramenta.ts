import { z } from 'zod';
import type { Ferramenta } from '@/catalogo/contratos';
import {
  type Cabecalho,
  cabecalhosEmLista,
  lerAmostra,
  listaEmCabecalhos,
  textoDaAmostra,
  validarCodigo,
} from './contratos';

type Definicao = Ferramenta['definicoes'][number];
type Conexao = NonNullable<Ferramenta['conexao']>;
// Os tipos do react-hook-form não instanciam sobre JSON recursivo (TS2589): no formulário o
// padrão do ajuste é opaco, e amostra e schema ficam de fora (a amostra é editada como texto).
// Cabeçalhos são uma lista editável aqui (o mapa {chave: valor} só existe na borda da API):
// mantendo o mapa como valor do campo, cada linha com chave vazia enquanto o usuário digita
// desaparecia a cada tecla.
export type RecursoNoFormulario = Omit<
  Ferramenta,
  'amostra' | 'schema' | 'definicoes' | 'conexao'
> & {
  definicoes: (Omit<Definicao, 'default'> & { default?: unknown })[];
  conexao: Omit<Conexao, 'headers'> & { headers: Cabecalho[] };
};

const ValorJson = z.json();

function paraFormulario(inicial: Ferramenta): RecursoNoFormulario {
  const { conexao, amostra, schema, definicoes, ...resto } = inicial;
  return {
    ...resto,
    definicoes,
    conexao: { ...conexao, headers: cabecalhosEmLista(conexao?.headers) },
  };
}

/** Devolve ao recurso o que ficou fora do formulário; o padrão veio do servidor, então é JSON. */
export function recursoCompleto(inicial: Ferramenta, recurso: RecursoNoFormulario): Ferramenta {
  const { conexao, ...resto } = recurso;
  return {
    ...inicial,
    ...resto,
    conexao: { ...conexao, headers: listaEmCabecalhos(conexao.headers) },
    definicoes: recurso.definicoes.map(({ default: padrao, ...definicao }) => ({
      ...definicao,
      ...(padrao === undefined ? {} : { default: ValorJson.parse(padrao) }),
    })),
  };
}

export type Valores = { recurso: RecursoNoFormulario; textoAmostra: string };

export const valoresIniciais = (inicial: Ferramenta, ferramenta: Ferramenta | null): Valores => ({
  recurso: paraFormulario(inicial),
  textoAmostra: ferramenta ? textoDaAmostra(ferramenta) : '',
});

// O recurso chega já validado pela borda da API; aqui só valem as regras do que se digita.
// O código só é validado quando muda: ferramentas antigas fora do padrão grupo.acao continuam
// salvando enquanto ninguém editar o código delas.
export function criarEsquemaDoFormulario(original: string) {
  return z
    .object({ recurso: z.custom<RecursoNoFormulario>(), textoAmostra: z.string() })
    .superRefine(({ recurso, textoAmostra }, contexto) => {
      if (recurso.titulo === undefined || recurso.titulo.trim() === '') {
        contexto.addIssue({
          code: 'custom',
          path: ['recurso', 'titulo'],
          message: 'Informe o título.',
        });
      }
      const codigo = recurso.name.trim();
      if (codigo !== original || original === '') {
        const erro = validarCodigo(codigo);
        if (erro) contexto.addIssue({ code: 'custom', path: ['recurso', 'name'], message: erro });
      }
      if (!lerAmostra(textoAmostra).ok) {
        contexto.addIssue({ code: 'custom', path: ['textoAmostra'], message: 'JSON inválido' });
      }
    });
}

export const rotuloDoCampo = (rotulo: string) => ({ htmlInput: { 'aria-label': rotulo } });
