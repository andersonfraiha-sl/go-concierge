import { TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller, type FieldPath, type FieldValues } from 'react-hook-form';
import { CAMPO } from '@/theme/tokens';

type Props<T extends FieldValues> = {
  control: Control<T>;
  name: FieldPath<T>;
  rotulo: string;
  editavel: boolean;
  minLinhas?: number;
  sx?: object;
};

/**
 * Texto simples, sem barra de formatação: o board da página Concierge usa um textbox comum
 * (o `CampoRico` com toolbar é do editor de skill, fora de escopo aqui).
 */
export function CampoDeTextoSimples<T extends FieldValues>({
  control,
  name,
  rotulo,
  editavel,
  minLinhas = 4,
  sx,
}: Props<T>): ReactNode {
  return (
    <Controller
      control={control}
      name={name}
      render={({ field }) => (
        <TextField
          {...field}
          value={field.value ?? ''}
          multiline
          minRows={minLinhas}
          fullWidth
          slotProps={{
            htmlInput: {
              readOnly: !editavel,
              'aria-readonly': String(!editavel),
              'aria-label': rotulo,
            },
          }}
          variant="outlined"
          sx={[
            {
              '& .MuiOutlinedInput-root': {
                borderRadius: `${CAMPO.raio}px`,
                padding: `${CAMPO.paddingY}px ${CAMPO.paddingX}px`,
              },
              '& .MuiOutlinedInput-notchedOutline': { borderColor: CAMPO.bordaEmRepouso },
            },
            sx ?? {},
          ]}
        />
      )}
    />
  );
}
