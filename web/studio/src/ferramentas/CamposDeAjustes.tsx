import { TextField } from '@mui/material';
import type { ReactNode } from 'react';
import { type Control, Controller } from 'react-hook-form';
import { padraoDoAjuste, TIPOS_DE_AJUSTE } from './contratos';
import { type RecursoNoFormulario, rotuloDoCampo, type Valores } from './formularioDaFerramenta';
import { ListaEditavel } from './ListaEditavel';

type Props = { control: Control<Valores>; travado: boolean };

const textoDoPadrao = (valor: unknown): string =>
  valor === undefined || valor === null ? '' : String(valor);

export function CamposDeAjustes({ control, travado }: Props): ReactNode {
  return (
    <Controller
      control={control}
      name="recurso.definicoes"
      render={({ field }) => (
        <ListaEditavel<RecursoNoFormulario['definicoes'][number]>
          titulo="Ajustes por condomínio"
          adicionar="Adicionar ajuste"
          linhas={field.value}
          desabilitado={travado}
          criarLinha={() => ({ name: '', label: '', type: 'string', options: [] })}
          onMudar={field.onChange}
          renderLinha={(d, mudarLinha) => (
            <>
              <TextField
                size="small"
                placeholder="prazo_dias"
                value={d.name}
                disabled={travado}
                slotProps={rotuloDoCampo('Nome do ajuste')}
                onChange={(e) => mudarLinha({ name: e.target.value })}
              />
              <TextField
                size="small"
                placeholder="Como aparece para o condomínio"
                value={d.label}
                disabled={travado}
                slotProps={rotuloDoCampo('Rótulo do ajuste')}
                onChange={(e) => mudarLinha({ label: e.target.value })}
              />
              <TextField
                select
                size="small"
                value={d.type ?? 'string'}
                disabled={travado}
                slotProps={{ ...rotuloDoCampo('Tipo do ajuste'), select: { native: true } }}
                onChange={(e) => mudarLinha({ type: e.target.value })}
              >
                {Object.entries(TIPOS_DE_AJUSTE).map(([valor, rotulo]) => (
                  <option key={valor} value={valor}>
                    {rotulo}
                  </option>
                ))}
              </TextField>
              <TextField
                size="small"
                placeholder="Valor quando o condomínio não decide"
                value={textoDoPadrao(d.default)}
                disabled={travado}
                slotProps={rotuloDoCampo('Padrão do ajuste')}
                onChange={(e) => mudarLinha({ default: padraoDoAjuste(d.type, e.target.value) })}
              />
              {d.type === 'select' && (
                <TextField
                  size="small"
                  placeholder="Opções, separadas por vírgula"
                  value={d.options.join(',')}
                  disabled={travado}
                  slotProps={rotuloDoCampo('Opções do ajuste')}
                  onChange={(e) => mudarLinha({ options: e.target.value.split(',') })}
                />
              )}
              <TextField
                size="small"
                fullWidth
                placeholder="Para que serve"
                value={d.description ?? ''}
                disabled={travado}
                slotProps={rotuloDoCampo('Descrição do ajuste')}
                onChange={(e) => mudarLinha({ description: e.target.value })}
              />
            </>
          )}
        />
      )}
    />
  );
}
