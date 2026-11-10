import { screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { estadoFalso, gestor } from '@/condominios/estadoFalso';
import { json, responderCom } from '@/test/fetchFalso';
import { LocalizacaoAtual } from '@/test/LocalizacaoAtual';
import { renderComProvedores } from '@/test/render';
import { SeletorDeCamada } from './SeletorDeCamada';

function montar() {
  responderCom({
    'GET /api/eu': () => json(gestor),
    'GET /api/admin/estado': () => json(estadoFalso),
  });
  return renderComProvedores(
    <>
      <SeletorDeCamada skill="reserva-do-salao" />
      <LocalizacaoAtual />
    </>,
    { rota: '/skills/reserva-do-salao', caminho: '/skills/*' },
  );
}

describe('SeletorDeCamada', () => {
  it('listsOwnAdministradoraAndCondominios', async () => {
    const { user } = montar();
    await user.click(await screen.findByRole('combobox', { name: 'Versão' }));
    const opcoes = within(await screen.findByRole('listbox')).getAllByRole('option');
    await screen.findByRole('option', { name: 'Vila Verde' });
    expect(opcoes.map((opcao) => opcao.textContent)).toEqual(
      expect.arrayContaining(['Superlógica', 'Sua administradora']),
    );
    expect(screen.getByRole('option', { name: 'Superlógica' })).toHaveAttribute(
      'aria-selected',
      'true',
    );
  });

  it('navigatesToLayerRoute', async () => {
    const { user } = montar();
    await user.click(await screen.findByRole('combobox', { name: 'Versão' }));
    await user.click(await screen.findByRole('option', { name: 'Vila Verde' }));
    expect(screen.getByTestId('localizacao')).toHaveTextContent(
      '/skills/reserva-do-salao/condominios/vila-verde',
    );
  });

  it('navigatesToOwnAdministradora', async () => {
    const { user } = montar();
    await user.click(await screen.findByRole('combobox', { name: 'Versão' }));
    await user.click(await screen.findByRole('option', { name: 'Sua administradora' }));
    expect(screen.getByTestId('localizacao')).toHaveTextContent(
      '/skills/reserva-do-salao/administradoras/adm-exemplo',
    );
  });
});
