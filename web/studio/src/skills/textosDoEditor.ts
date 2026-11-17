import { z } from 'zod';
import type { Textos } from './contratos';

const aparadasSemVazias = (itens: string[]): string[] =>
  itens.map((i) => i.trim()).filter((i) => i !== '');

/** Frase ou palavra em branco é rascunho na tela, não vai para o servidor. */
export const semVazios = (textos: Textos): Textos => ({
  ...textos,
  palavrasChave: aparadasSemVazias(textos.palavrasChave),
  frasesDeExemplo: aparadasSemVazias(textos.frasesDeExemplo),
});

const TextosPendentesSchema = z.object({ resumo: z.string(), quandoUsar: z.string() });
export type TextosPendentes = z.infer<typeof TextosPendentesSchema>;
const EstadoDaRotaSchema = z
  .object({ textosPendentes: TextosPendentesSchema.optional() })
  .nullish();

/** Textos digitados ao criar a skill que não chegaram a salvar (diálogo "Nova skill" com PUT falho). */
export const textosPendentesDaRota = (estadoDaRota: unknown): TextosPendentes | undefined => {
  const resultado = EstadoDaRotaSchema.safeParse(estadoDaRota);
  return resultado.success ? resultado.data?.textosPendentes : undefined;
};
