import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { AdministradorasAba } from './AdministradorasAba';
import { construtor, rotasBase } from './estadoFalso';
import { gravador } from './paginaDeTeste';

const ADMINISTRADORA = { id: 'adm-exemplo', nome: 'Administradora Exemplo' };

function condominio(id: string) {
  return {
    id,
    nome: id,
    administradora_id: 'adm-exemplo',
    account: 'acc-1',
    company_id: 'gruvi-1',
    tenant: 'atende-1',
  };
}

function montar(condominios: ReturnType<typeof condominio>[] = []) {
  responderCom(rotasBase(construtor));
  return renderComProvedores(
    <AdministradorasAba administradoras={[ADMINISTRADORA]} condominios={condominios} />,
  );
}

describe('AdministradorasAba', () => {
  it('renamesAdministradoraInPlace', async () => {
    const { corpos, handlers } = gravador('PUT /api/admin/administradora');
    responderCom({ ...rotasBase(construtor), ...handlers });
    const { user } = renderComProvedores(
      <AdministradorasAba administradoras={[ADMINISTRADORA]} condominios={[]} />,
    );

    await user.click(
      await screen.findByRole('button', {
        name: 'Editar nome da administradora Administradora Exemplo',
      }),
    );
    const campo = screen.getByRole('textbox', {
      name: 'nome da administradora Administradora Exemplo',
    });
    await user.clear(campo);
    await user.type(campo, 'Nova{Enter}');

    expect(await screen.findByText('Administradora renomeada.')).toBeInTheDocument();
    expect(corpos).toEqual([{ id: 'adm-exemplo', nome: 'Nova' }]);
  });

  it('identifiesEachRowByItsOwnAdministradora', async () => {
    responderCom(rotasBase(construtor));
    renderComProvedores(
      <AdministradorasAba
        administradoras={[ADMINISTRADORA, { id: 'adm-outra', nome: 'Outra Administradora' }]}
        condominios={[]}
      />,
    );

    expect(
      await screen.findByRole('button', {
        name: 'Editar nome da administradora Administradora Exemplo',
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Editar nome da administradora Outra Administradora' }),
    ).toBeInTheDocument();
  });

  it.each([
    { quantidade: 0, esperado: 'Nenhum condomínio' },
    { quantidade: 1, esperado: '1 condomínio' },
    { quantidade: 3, esperado: '3 condomínios' },
  ])('showsCondominioCount($quantidade)', async ({ quantidade, esperado }) => {
    const condominios = Array.from({ length: quantidade }, (_, i) => condominio(`c${i}`));
    montar(condominios);

    expect(await screen.findByText(esperado)).toBeInTheDocument();
  });

  it('linksToFilteredList', async () => {
    montar();

    const link = await screen.findByRole('link', { name: 'Nenhum condomínio' });
    expect(link).toHaveAttribute('href', '/condominios?aba=condominios&administradora=adm-exemplo');
  });

  it('showsId', async () => {
    montar();

    expect(await screen.findByText('adm-exemplo')).toBeInTheDocument();
  });
});
