import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { AdministradorasPage } from './AdministradorasPage';
import { construtor, estadoFalso, rotasBase } from './estadoFalso';

describe('AdministradorasPage', () => {
  it('showsAdministradorasForm', async () => {
    responderCom(rotasBase(construtor));
    renderComProvedores(<AdministradorasPage />);

    expect(
      await screen.findByRole('textbox', { name: 'Nome da administradora' }),
    ).toBeInTheDocument();
    expect(screen.queryByText('Nenhum condomínio ainda.')).not.toBeInTheDocument();
  });

  it('removesAdministradoraAfterConfirmation', async () => {
    let removida = '';
    const comVazia = structuredClone(estadoFalso);
    comVazia.administradoras.push({ id: 'adm-vazia', nome: 'Administradora Vazia' });
    responderCom({
      ...rotasBase(construtor),
      'GET /api/admin/estado': () => json(comVazia),
      'DELETE /api/admin/administradora': (req) => {
        removida = new URL(req.url).searchParams.get('id') ?? '';
        return json({
          mapeamento: estadoFalso.mapeamento,
          administradoras: estadoFalso.administradoras,
        });
      },
    });
    const { user } = renderComProvedores(<AdministradorasPage />);

    await user.click(await screen.findByRole('button', { name: 'Remover Administradora Vazia' }));
    expect(
      screen.getByText('Remover "Administradora Vazia"? Não dá para desfazer.'),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Remover' }));

    await screen.findByText('Administradora removida.');
    expect(removida).toBe('adm-vazia');
  });

  it('blocksRemovingAdministradoraWithCondominios', async () => {
    responderCom(rotasBase(construtor));
    renderComProvedores(<AdministradorasPage />);

    expect(
      await screen.findByRole('button', { name: 'Remover Administradora Exemplo' }),
    ).toBeDisabled();
    expect(screen.getByText('Mova os condomínios dela antes de remover.')).toBeInTheDocument();
  });
});
