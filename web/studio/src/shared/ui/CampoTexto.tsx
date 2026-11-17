import { TextField, type TextFieldProps } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller, type FieldPath, type FieldValues } from 'react-hook-form';

type Props<T extends FieldValues> = Omit<
  TextFieldProps,
  'name' | 'value' | 'onChange' | 'error' | 'inputRef'
> & {
  control: Control<T>;
  name: FieldPath<T>;
};

/** TextField ligado ao react-hook-form: o erro do schema aparece no lugar do helperText. */
export function CampoTexto<T extends FieldValues>({
  control,
  name,
  helperText,
  onBlur,
  ...props
}: Props<T>): ReactNode {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field, fieldState }) => (
        <TextField
          {...props}
          name={field.name}
          value={field.value ?? ''}
          onChange={field.onChange}
          onBlur={(evento) => {
            field.onBlur();
            onBlur?.(evento);
          }}
          inputRef={field.ref}
          error={fieldState.error !== undefined}
          helperText={fieldState.error?.message ?? helperText}
        />
      )}
    />
  );
}
