import { type Control, type FieldValues, useWatch } from 'react-hook-form';

/**
 * Valores do formulário com referência nova só quando o conteúdo muda: é o `valor` do autosave.
 * Clona porque o react-hook-form entrega o objeto vivo, que muda no lugar a cada tecla.
 */
export function useValoresDoFormulario<T extends FieldValues>(control: Control<T>): T {
  return useWatch({ control, compute: (valores: T) => structuredClone(valores) });
}
