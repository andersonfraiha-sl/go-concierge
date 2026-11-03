import { Checkbox, FormControlLabel, TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller } from 'react-hook-form';
import type { Ferramenta } from '@/catalogo/contratos';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { rotuloDoCampo, type Valores } from './formularioDaFerramenta';
import { ListaEditavel } from './ListaEditavel';

const TIPOS_DE_DADO = ['string', 'number', 'boolean', 'date', 'array', 'object'];

type Props = { control: Control<Valores>; travado: boolean };

export function CamposDeParametros({ control, travado }: Props): ReactNode {
  return (
    <Controller
      control={control}
      name="recurso.params"
      render={({ field }) => (
        <ListaEditavel<Ferramenta['params'][number]>
          titulo="Parâmetros"
          adicionar="Adicionar parâmetro"
          linhas={field.value}
          desabilitado={travado}
          criarLinha={() => ({ name: '', type: 'string', required: false })}
          onMudar={field.onChange}
          renderLinha={(p, mudarLinha) => (
            <>
              <TextField
                size="small"
                placeholder="unidade_id"
                value={p.name}
                disabled={travado}
                slotProps={rotuloDoCampo('Nome do parâmetro')}
                onChange={(e) => mudarLinha({ name: e.target.value })}
              />
              <TextField
                select
                size="small"
                value={p.type ?? 'string'}
                disabled={travado}
                slotProps={{ ...rotuloDoCampo('Tipo do parâmetro'), select: { native: true } }}
                onChange={(e) => mudarLinha({ type: e.target.value })}
              >
                {TIPOS_DE_DADO.map((tipo) => (
                  <option key={tipo} value={tipo}>
                    {tipo}
                  </option>
                ))}
              </TextField>
              <FormControlLabel
                label="Obrigatório"
                control={
                  <Checkbox
                    size="small"
                    checked={p.required ?? false}
                    disabled={travado}
                    onChange={(e) => mudarLinha({ required: e.target.checked })}
                  />
                }
              />
              <TextField
                size="small"
                fullWidth
                placeholder="O que é, formato, exemplo"
                value={p.description ?? ''}
                disabled={travado}
                slotProps={rotuloDoCampo('Descrição do parâmetro')}
                onChange={(e) => mudarLinha({ description: e.target.value })}
              />
            </>
          )}
        />
      )}
    />
  );
}

export function CamposDeSaida({ control, travado }: Props): ReactNode {
  return (
    <>
      <Controller
        control={control}
        name="recurso.saida"
        render={({ field }) => (
          <ListaEditavel<Ferramenta['saida'][number]>
            titulo="Saída"
            adicionar="Adicionar campo de saída"
            linhas={field.value}
            desabilitado={travado}
            criarLinha={() => ({ name: '' })}
            onMudar={field.onChange}
            renderLinha={(s, mudarLinha) => (
              <>
                <TextField
                  size="small"
                  placeholder="itens"
                  value={s.name}
                  disabled={travado}
                  slotProps={rotuloDoCampo('Nome do campo de saída')}
                  onChange={(e) => mudarLinha({ name: e.target.value })}
                />
                <TextField
                  select
                  size="small"
                  value={s.type ?? 'string'}
                  disabled={travado}
                  slotProps={{
                    ...rotuloDoCampo('Tipo do campo de saída'),
                    select: { native: true },
                  }}
                  onChange={(e) => mudarLinha({ type: e.target.value })}
                >
                  {TIPOS_DE_DADO.map((tipo) => (
                    <option key={tipo} value={tipo}>
                      {tipo}
                    </option>
                  ))}
                </TextField>
                <TextField
                  size="small"
                  placeholder="data.items[].id"
                  value={s.path ?? ''}
                  disabled={travado}
                  slotProps={rotuloDoCampo('Caminho do campo de saída')}
                  onChange={(e) => mudarLinha({ path: e.target.value })}
                />
                <TextField
                  size="small"
                  fullWidth
                  placeholder="O que o campo significa"
                  value={s.description ?? ''}
                  disabled={travado}
                  slotProps={rotuloDoCampo('Descrição do campo de saída')}
                  onChange={(e) => mudarLinha({ description: e.target.value })}
                />
              </>
            )}
          />
        )}
      />
      <CampoTexto
        control={control}
        name="recurso.output"
        label="Resultado principal"
        disabled={travado}
      />
    </>
  );
}
