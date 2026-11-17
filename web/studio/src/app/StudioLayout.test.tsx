import { screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { StudioLayout } from './StudioLayout';

const eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'condominios.editar', 'studio.avancado'],
  administradoraId: null,
  dev: false,
};

describe('StudioLayout', () => {
  beforeEach(() => {
    responderCom({ 'GET /api/eu': () => json(eu) });
  });

  it('mostraStudioNoTopoESeisItensDeNavegacao', async () => {
    renderComProvedores(<StudioLayout />);

    expect(screen.getByText('Studio')).toBeInTheDocument();
    await screen.findByRole('link', { name: 'Casos de uso' });
    const rotulos = [
      'Concierge',
      'Casos de uso',
      'Recursos',
      'Condomínios',
      'Avançado',
      'Atendente',
    ];
    for (const rotulo of rotulos) {
      expect(screen.getByRole('link', { name: rotulo })).toBeInTheDocument();
    }
  });

  it('escondeAdministradorasSemPermissao', async () => {
    renderComProvedores(<StudioLayout />);

    await screen.findByRole('link', { name: 'Condomínios' });
    expect(screen.queryByRole('link', { name: 'Administradoras' })).not.toBeInTheDocument();
  });

  it('mostraAdministradorasComPermissaoDeEditar', async () => {
    const admin = { ...eu, permissoes: [...eu.permissoes, 'administradoras.editar'] };
    responderCom({ 'GET /api/eu': () => json(admin) });
    renderComProvedores(<StudioLayout />);

    expect(await screen.findByRole('link', { name: 'Administradoras' })).toHaveAttribute(
      'href',
      '/administradoras',
    );
  });

  it('escondeCondominiosEAdministradorasPelaChaveELembra', async () => {
    const admin = { ...eu, permissoes: [...eu.permissoes, 'administradoras.editar'] };
    responderCom({ 'GET /api/eu': () => json(admin) });
    const { user } = renderComProvedores(<StudioLayout />);
    await screen.findByRole('link', { name: 'Administradoras' });

    await user.click(screen.getByRole('switch', { name: 'Condomínios e administradoras' }));

    expect(screen.queryByRole('link', { name: 'Condomínios' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Administradoras' })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Recursos' })).toBeInTheDocument();
    expect(localStorage.getItem('studio.semCondominios')).toBe('true');
  });

  it('mostraCondominioNoSingularEAbreDiretoNeleParaQuemSoTemUm', async () => {
    const gestorDoCondominio = {
      ...eu,
      papel: 'gestor_administradora',
      permissoes: ['skills.ler', 'condominios.habilitar'],
      administradoraId: 'adm-a',
      condominioId: 'vila verde',
    };
    responderCom({ 'GET /api/eu': () => json(gestorDoCondominio) });
    renderComProvedores(<StudioLayout />);

    expect(await screen.findByRole('link', { name: 'Condomínio' })).toHaveAttribute(
      'href',
      '/condominios/vila%20verde',
    );
    expect(screen.queryByRole('link', { name: 'Condomínios' })).not.toBeInTheDocument();
    expect(screen.getByText('Gestor · Condomínio')).toBeInTheDocument();
  });

  it('mantemCondominiosNoPluralParaQuemVeAAdministradoraInteira', async () => {
    const gestor = { ...eu, papel: 'gestor_administradora', administradoraId: 'adm-a' };
    responderCom({ 'GET /api/eu': () => json(gestor) });
    renderComProvedores(<StudioLayout />);

    expect(await screen.findByRole('link', { name: 'Condomínios' })).toHaveAttribute(
      'href',
      '/condominios',
    );
    expect(screen.getByText('Gestor · Administradora')).toBeInTheDocument();
  });

  it('apontaAtendenteParaOConsoleForaDoSpa', () => {
    renderComProvedores(<StudioLayout />);
    expect(screen.getByRole('link', { name: 'Atendente' })).toHaveAttribute('href', '/console');
  });

  it('iniciaRecolhidoQuandoHaPreferenciaGravada', () => {
    localStorage.setItem('studio.menuRecolhido', 'true');

    renderComProvedores(<StudioLayout />);

    expect(screen.getByRole('button', { name: 'Expandir menu' })).toBeInTheDocument();
  });

  it('recolheOMenuELembraAPreferencia', async () => {
    const { user } = renderComProvedores(<StudioLayout />);

    await user.click(screen.getByRole('button', { name: 'Recolher menu' }));

    expect(screen.getByRole('button', { name: 'Expandir menu' })).toBeInTheDocument();
    expect(localStorage.getItem('studio.menuRecolhido')).toBe('true');
  });

  it('showsLoadingTextWhileEuIsPending', () => {
    responderCom({ 'GET /api/eu': () => new Promise(() => {}) });

    renderComProvedores(<StudioLayout />);

    expect(screen.getByText('Carregando…')).toBeInTheDocument();
  });

  it('showsUnrecognizedUserScreenOn401InsteadOfTheLayout', async () => {
    responderCom({ 'GET /api/eu': () => json({ dev: false }, 401) });

    renderComProvedores(<StudioLayout />);

    expect(await screen.findByRole('alert')).toHaveTextContent('Não reconhecemos seu usuário.');
    expect(screen.queryByRole('navigation', { name: 'Menu principal' })).not.toBeInTheDocument();
  });

  it('placesCopilotLeftOfContent', async () => {
    localStorage.setItem('studio.copilotoAberto', 'true');
    responderCom({
      'GET /api/eu': () => json(eu),
      'GET /api/copiloto/estado': () => json({ disponivel: true }),
    });

    renderComProvedores(<StudioLayout />);

    const painel = await screen.findByRole('complementary', { name: 'Copiloto' });
    const conteudo = screen.getByRole('main');
    const menu = screen.getByRole('navigation', { name: 'Menu principal' });
    expect(menu.compareDocumentPosition(painel)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(painel.compareDocumentPosition(conteudo)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });
});
