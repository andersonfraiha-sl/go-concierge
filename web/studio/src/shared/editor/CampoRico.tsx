import { Skeleton } from '@mui/material';
import { lazy, type ReactNode, Suspense } from 'react';
import { type Control, Controller, type FieldPath, type FieldValues } from 'react-hook-form';
import type { PropsDoEditorRico } from './EditorRico';

// O editor (TipTap) vem num pedaço à parte: telas sem texto longo não o baixam.
const EditorRico = lazy(() =>
  import('./EditorRico').then((modulo) => ({ default: modulo.EditorRico })),
);

type Props<T extends FieldValues> = Omit<PropsDoEditorRico, 'valor' | 'onChange'> & {
  control: Control<T>;
  name: FieldPath<T>;
};

/** Texto longo em Markdown com barra de formatação, ligado ao react-hook-form. */
export function CampoRico<T extends FieldValues>({ control, name, ...props }: Props<T>): ReactNode {
  const altura = `${(props.minLinhas ?? 4) * 1.6 + 4}em`;
  return (
    <Controller
      control={control}
      name={name}
      render={({ field }) => (
        <Suspense
          fallback={
            <Skeleton variant="rounded" height={altura} aria-label={`Carregando ${props.rotulo}`} />
          }
        >
          <EditorRico valor={field.value ?? ''} onChange={field.onChange} {...props} />
        </Suspense>
      )}
    />
  );
}
