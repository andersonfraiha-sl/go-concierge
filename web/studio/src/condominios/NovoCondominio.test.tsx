import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { EstadoSchema } from './contratos';
import { construtor, estadoFalso, rotasBase } from './estadoFalso';
import { NovoCondominio } from './NovoCondominio';
import { gravador } from './paginaDeTeste';

const estado = EstadoSchema.parse({
  ...estadoFalso,
  mapeamento: { ...estadoFalso.mapeamento, condominios: [] },
});

async function preencherESalvar(
  rotas: Record<string, (req: Request) => Response | Promise<Response>>,
) {
  responderCom({ ...rotasBase(construtor), ...rotas });
  const { user } = renderComProvedores(
    <NovoCondominio aberto estado={estado} onFechar={() => undefined} />,
  );
  await user.type(await screen.findByLabelText('Nome'), 'Cond X');
  await user.click(screen.getByRole('button', { name: 'Criar condomínio' }));
}

describe('NovoCondominio', () => {
  it('createsCondominioWithPost', async () => {
    const { corpos, handlers } = gravador('POST /api/admin/condominio');
    await preencherESalvar(handlers);

    expect(await screen.findByText('Condomínio criado.')).toBeInTheDocument();
    expect(corpos[0]).toMatchObject({ id: 'cond-x', nome: 'Cond X' });
  });

  it('showsConflictMessageOnDuplicateCondominio', async () => {
    await preencherESalvar({
      'POST /api/admin/condominio': () =>
        json({ codigo: 'JA_EXISTE', error: 'condomínio cond-x já existe' }, 409),
    });

    expect(
      await screen.findByText('Já existe um condomínio com esse identificador.'),
    ).toBeInTheDocument();
  });
});
