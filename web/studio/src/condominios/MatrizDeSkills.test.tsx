import { screen, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { useEstado } from './contratos';
import { construtor, estadoFalso, gestor, rotasBase } from './estadoFalso';
import { MatrizDeSkills } from './MatrizDeSkills';
import { gravador } from './paginaDeTeste';

/** Vila Verde ganha decisão própria divergente do Padrão, para marcar o ponto na matriz. */
const estadoComDecisaoPropria = structuredClone(estadoFalso);
estadoComDecisaoPropria.mapeamento.habilitacoes['vila-verde'] = {
  'agendar-mudanca': { habilitado: false, papeis: [] },
};

function Harness({ editavel = true }: { editavel?: boolean }): ReactNode {
  const consulta = useEstado();
  if (!consulta.data) return null;
  const titulos = new Map(consulta.data.casos.map((caso) => [caso.nome, caso.titulo]));
  return <MatrizDeSkills estado={consulta.data} titulos={titulos} editavel={editavel} />;
}

const abrirMatriz = (opcoes: { editavel?: boolean } = {}) =>
  renderComProvedores(<Harness {...opcoes} />);

describe('MatrizDeSkills', () => {
  it('togglesCellForCondominio', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/habilitacao');
    responderCom({ ...rotasBase(construtor), ...handlers });
    const { user } = abrirMatriz();

    await user.click(await screen.findByRole('switch', { name: 'Agendar mudança em Vila Verde' }));

    await waitFor(() =>
      expect(corpos[0]).toMatchObject({
        condominio: 'vila-verde',
        caso: 'agendar-mudanca',
        habilitado: false,
      }),
    );
  });

  it('marksOwnDecision', async () => {
    responderCom({
      ...rotasBase(construtor),
      'GET /api/admin/estado': () => json(estadoComDecisaoPropria),
    });
    abrirMatriz();

    await screen.findByRole('switch', { name: 'Agendar mudança em Vila Verde' });
    expect(screen.getAllByRole('img', { name: 'Decisão deste condomínio' }).length).toBeGreaterThan(
      0,
    );
  });

  it('opensDrawerFromCell', async () => {
    responderCom(rotasBase(construtor));
    const { user } = abrirMatriz();

    const switchDaCelula = await screen.findByRole('switch', {
      name: 'Agendar mudança em Vila Verde',
    });
    const celula = switchDaCelula.closest('td');
    if (!celula) throw new Error('célula não encontrada');
    await user.click(celula);

    expect(await screen.findByRole('dialog', { name: /Agendar mudança/ })).toBeInTheDocument();
  });

  it('locksDefaultColumnForGestor', async () => {
    responderCom(rotasBase(gestor));
    abrirMatriz();

    expect(await screen.findByRole('switch', { name: 'Agendar mudança em Padrão' })).toBeDisabled();
    expect(screen.getByRole('switch', { name: 'Agendar mudança em Vila Verde' })).toBeEnabled();
  });
});
