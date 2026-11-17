import { screen } from '@testing-library/react';
import { useLocation } from 'react-router';
import { describe, expect, it, vi } from 'vitest';
import { construtor, rotasBase } from '@/condominios/estadoFalso';
import { responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { BuscaGlobal } from './BuscaGlobal';

function Caminho() {
  return <output aria-label="Caminho">{useLocation().pathname}</output>;
}

const leitor = { ...construtor, permissoes: [...construtor.permissoes, 'skills.ler'] };

const montar = () =>
  renderComProvedores(
    <>
      <BuscaGlobal />
      <Caminho />
    </>,
  );

describe('BuscaGlobal', () => {
  it.each([
    { termo: 'mudanca', opcao: 'Agendar mudança', destino: '/skills/agendar-mudanca' },
    { termo: 'vila', opcao: 'Vila Verde', destino: '/condominios/vila-verde' },
  ])('abre $opcao ao escolher', async ({ termo, opcao, destino }) => {
    responderCom(rotasBase(leitor));
    const { user } = montar();

    await user.type(screen.getByRole('combobox', { name: 'Buscar no Studio' }), termo);
    await user.click(await screen.findByRole('option', { name: opcao }));

    expect(screen.getByRole('status', { name: 'Caminho' })).toHaveTextContent(destino);
    expect(screen.getByRole('combobox', { name: 'Buscar no Studio' })).toHaveValue('');
  });

  it('naoMostraSkillSemPermissaoDeLer', async () => {
    responderCom(rotasBase(construtor));
    const { user } = montar();

    await user.type(screen.getByRole('combobox', { name: 'Buscar no Studio' }), 'a');

    expect(await screen.findByRole('option', { name: 'Vila Verde' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Agendar mudança' })).not.toBeInTheDocument();
  });

  it('naoBuscaListasAntesDeUsar', async () => {
    const falso = responderCom(rotasBase(leitor));
    montar();
    await vi.waitFor(() => expect(falso).toHaveBeenCalled());
    const caminhos = falso.mock.calls.map(([entrada]) => String(entrada));
    expect(caminhos).toEqual(['/api/eu']);
  });
});
