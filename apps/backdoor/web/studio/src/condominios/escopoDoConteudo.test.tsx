import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { EstadoSchema, useAplicarMapeamento, useEstado } from './contratos';
import { construtor, estadoFalso } from './estadoFalso';

function estadoDaEmpresa(company: string) {
  return EstadoSchema.parse({
    ...estadoFalso,
    mapeamento: {
      ...estadoFalso.mapeamento,
      orientacoes: { '101': { geral: company, por_caso: {} } },
    },
  });
}

function DuasEmpresas() {
  const alpha = useEstado({ escopo: 'alpha', condoID: '101' });
  const beta = useEstado({ escopo: 'beta', condoID: '101' });
  const aplicar = useAplicarMapeamento({ escopo: 'alpha', condoID: '101' });
  return (
    <>
      <output aria-label="Alpha">{alpha.data?.mapeamento.orientacoes['101']?.geral}</output>
      <output aria-label="Beta">{beta.data?.mapeamento.orientacoes['101']?.geral}</output>
      <button
        type="button"
        onClick={() =>
          aplicar({
            mapeamento: estadoDaEmpresa('alpha atualizada').mapeamento,
            administradoras: [],
          })
        }
      >
        Atualizar Alpha
      </button>
    </>
  );
}

describe('cache do conteúdo', () => {
  it('gravarEmAlphaNaoSubstituiOMesmoCondominioEmBeta', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/admin/estado': (req) =>
        json(estadoDaEmpresa(new URL(req.url).searchParams.get('company_id') ?? 'global')),
    });
    const { user } = renderComProvedores(<DuasEmpresas />);
    await screen.findByText('alpha');
    await screen.findByText('beta');

    await user.click(screen.getByRole('button', { name: 'Atualizar Alpha' }));

    expect(screen.getByRole('status', { name: 'Alpha' })).toHaveTextContent('alpha atualizada');
    expect(screen.getByRole('status', { name: 'Beta' })).toHaveTextContent('beta');
  });
});
