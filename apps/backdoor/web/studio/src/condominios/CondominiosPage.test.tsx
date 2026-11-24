import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { CondominiosPage } from './CondominiosPage';
import { construtor, estadoFalso, gestor, rotasBase } from './estadoFalso';

const estado = structuredClone(estadoFalso);
const condoBase = estadoFalso.mapeamento.condominios[0];
if (!condoBase) throw new Error('fixture sem condomínio');
estado.administradoras = [
  { id: 'company-a', nome: 'Empresa Alpha' },
  { id: 'company-b', nome: 'Empresa Beta' },
];
estado.mapeamento.condominios = [
  {
    ...condoBase,
    id: '101',
    nome: 'Condomínio Alpha',
    administradora_id: 'company-a',
  },
  {
    ...condoBase,
    id: '101',
    nome: 'Condomínio Beta',
    administradora_id: 'company-b',
  },
];

function montar(eu = construtor, rota = '/condominios') {
  responderCom({ ...rotasBase(eu), 'GET /api/admin/estado': () => json(estado) });
  return renderComProvedores(<CondominiosPage />, { rota });
}

describe('alvos de conteúdo', () => {
  it('listaGlobalEmpresaECondominioSemCadastroOuHabilitacao', async () => {
    montar();

    expect(await screen.findByRole('link', { name: 'Conteúdo global' })).toHaveAttribute(
      'href',
      '/condominios/*',
    );
    expect(screen.getByRole('link', { name: /Empresa Alpha Conteúdo da empresa/ })).toHaveAttribute(
      'href',
      '/condominios/*?company_id=company-a',
    );
    expect(screen.getByRole('heading', { name: 'Conteúdo' })).toBeInTheDocument();
    expect(
      screen.queryByRole('button', { name: /Novo condomínio|Criar|Remover/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.queryByRole('navigation', { name: 'Tela antiga' })).not.toBeInTheDocument();
  });

  it('preservaEmpresaNaNavegacaoDeCondominiosComMesmoId', async () => {
    montar();

    expect(await screen.findByRole('link', { name: /Condomínio Alpha/ })).toHaveAttribute(
      'href',
      '/condominios/101?company_id=company-a',
    );
    expect(screen.getByRole('link', { name: /Condomínio Beta/ })).toHaveAttribute(
      'href',
      '/condominios/101?company_id=company-b',
    );
  });

  it('naoOfereceConteudoGlobalNoEscopoDaEmpresa', async () => {
    montar(gestor, '/condominios?company_id=company-a');

    await screen.findByRole('link', { name: /Condomínio Alpha/ });
    expect(screen.queryByRole('link', { name: 'Conteúdo global' })).not.toBeInTheDocument();
  });

  it('permiteRepetirLeituraQueFalhou', async () => {
    let chamadas = 0;
    responderCom({
      ...rotasBase(construtor),
      'GET /api/admin/estado': () =>
        ++chamadas === 1 ? json({ codigo: 'ERRO', mensagem: 'caiu' }, 500) : json(estado),
    });
    const { user } = renderComProvedores(<CondominiosPage />);

    await user.click(await screen.findByRole('button', { name: 'Tentar de novo' }));

    expect(await screen.findByRole('link', { name: /Condomínio Alpha/ })).toBeInTheDocument();
  });
});
