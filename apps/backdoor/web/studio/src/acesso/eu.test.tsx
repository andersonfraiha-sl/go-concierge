import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { StudioLayout } from '@/app/StudioLayout';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { type Eu, pode, podeNoCatalogo, useEu, usePodeEditar, usePodeEditarCondominio } from './eu';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos', 'skills.editar_fluxo'],
  administradoraId: null,
  dev: true,
};
const gestor: Eu = {
  ...construtor,
  papel: 'gestor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
  administradoraId: 'adm-1',
};
const gestorAvancado: Eu = {
  ...gestor,
  papel: 'gestor_avancado',
  permissoes: [...gestor.permissoes, 'studio.avancado'],
};

describe('permissões', () => {
  it.each([
    { eu: construtor, permissao: 'skills.editar_fluxo', pode: true, noCatalogo: true },
    { eu: gestor, permissao: 'skills.editar_textos', pode: true, noCatalogo: false },
    { eu: gestor, permissao: 'studio.avancado', pode: false, noCatalogo: false },
    { eu: gestorAvancado, permissao: 'studio.avancado', pode: true, noCatalogo: false },
  ] as const)('$eu.papel / $permissao: pode=$pode noCatalogo=$noCatalogo', (caso) => {
    expect(pode(caso.eu, caso.permissao)).toBe(caso.pode);
    expect(podeNoCatalogo(caso.eu, caso.permissao)).toBe(caso.noCatalogo);
  });
});

function Probe() {
  const { data: eu } = useEu();
  const edita = usePodeEditar('skills.editar_fluxo');
  const editaCondominio = usePodeEditarCondominio('condominios.habilitar');
  if (!eu) return <span>carregando</span>;
  return (
    <span>{`${eu.nome}: ${edita ? 'edita' : 'le'} / ${editaCondominio ? 'edita' : 'le'}`}</span>
  );
}

function simularTela(estreita: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: estreita && query === '(max-width:899.95px)',
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

describe('permissões de edição por largura e escopo', () => {
  const autorizado = {
    ...construtor,
    permissoes: [...construtor.permissoes, 'condominios.habilitar'],
  };
  const vinculado = { ...autorizado, administradoraId: 'adm-1' };
  const leitor = { ...autorizado, permissoes: ['skills.ler'] };

  it.each([true, false])('mantém as permissões com tela estreita=%s', async (estreita) => {
    simularTela(estreita);
    for (const [eu, esperado] of [
      [autorizado, 'Ana: edita / edita'],
      [vinculado, 'Ana: le / edita'],
      [leitor, 'Ana: le / le'],
    ] as const) {
      responderCom({ 'GET /api/eu': () => json(eu) });
      const tela = renderComProvedores(<Probe />);
      expect(await screen.findByText(esperado)).toBeInTheDocument();
      tela.unmount();
    }
  });
});

describe('useEu sem sessão', () => {
  it('showsUnauthenticatedScreenInLayout', async () => {
    responderCom({ 'GET /api/eu': () => json({ codigo: 'NAO_AUTENTICADO', error: 'x' }, 401) });
    renderComProvedores(<StudioLayout />);
    expect(await screen.findByText('Não reconhecemos seu usuário.')).toBeInTheDocument();
  });
});
