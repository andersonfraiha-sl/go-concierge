import { z } from 'zod';

/** O Go serializa slice nil como null: a borda entrega sempre lista. */
export const listaDoGo = <S extends z.ZodType>(item: S) =>
  z
    .array(item)
    .nullish()
    .transform((lista) => lista ?? []);
