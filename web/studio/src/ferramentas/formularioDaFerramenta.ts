import { z } from 'zod';
import type { Ferramenta } from '@/catalogo/contratos';
import { lerAmostra, textoDaAmostra } from './contratos';

type Definicao = Ferramenta['definicoes'][number];
// Os tipos do react-hook-form não instanciam sobre JSON recursivo (TS2589): no formulário o
// padrão do ajuste é opaco, e amostra e schema ficam de fora (a amostra é editada como texto).
export type RecursoNoFormulario = Omit<Ferramenta, 'amostra' | 'schema' | 'definicoes'> & {
  definicoes: (Omit<Definicao, 'default'> & { default?: unknown })[];
};

const ValorJson = z.json();

/** Devolve ao recurso o que ficou fora do formulário; o padrão veio do servidor, então é JSON. */
export function recursoCompleto(inicial: Ferramenta, recurso: RecursoNoFormulario): Ferramenta {
  return {
    ...inicial,
    ...recurso,
    definicoes: recurso.definicoes.map(({ default: padrao, ...definicao }) => ({
      ...definicao,
      ...(padrao === undefined ? {} : { default: ValorJson.parse(padrao) }),
    })),
  };
}

export type Valores = { recurso: RecursoNoFormulario; textoAmostra: string };

export const valoresIniciais = (inicial: Ferramenta, ferramenta: Ferramenta | null): Valores => ({
  recurso: inicial,
  textoAmostra: ferramenta ? textoDaAmostra(ferramenta) : '',
});

// O recurso chega já validado pela borda da API; aqui só valem as regras do que se digita.
export const FormularioSchema = z
  .object({ recurso: z.custom<RecursoNoFormulario>(), textoAmostra: z.string() })
  .superRefine(({ recurso, textoAmostra }, contexto) => {
    if (recurso.name.trim() === '') {
      contexto.addIssue({ code: 'custom', path: ['recurso', 'name'], message: 'Informe o nome.' });
    }
    if (!lerAmostra(textoAmostra).ok) {
      contexto.addIssue({ code: 'custom', path: ['textoAmostra'], message: 'JSON inválido' });
    }
  });

export const rotuloDoCampo = (rotulo: string) => ({ htmlInput: { 'aria-label': rotulo } });
