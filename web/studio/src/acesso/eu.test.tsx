import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { StudioLayout } from '@/app/StudioLayout';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { type Eu, PERMISSOES, pode, podeNoCatalogo, useEu, usePodeEditar } from './eu';

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

describe('permissões das camadas', () => {
  it('listsLayerPermissionsRightAfterSkillsExcluir', () => {
    const depoisDeExcluir = PERMISSOES.slice(PERMISSOES.indexOf('skills.excluir') + 1);
    expect(depoisDeExcluir.slice(0, 3)).toEqual([
      'skills.editar_etapas',
      'skills.decidir_atualizacao',
      'skills.publicar_base',
    ]);
  });
});

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
  if (!eu) return <span>carregando</span>;
  return <span>{`${eu.nome}: ${edita ? 'edita' : 'le'}`}</span>;
}

function simularTela(estreita: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: estreita && query === '(max-width:899.95px)',
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

describe('usePodeEditar', () => {
  it('narrowScreenDisablesEditing', async () => {
    responderCom({ 'GET /api/eu': () => json(construtor) });
    simularTela(true);
    renderComProvedores(<Probe />);
    expect(await screen.findByText('Ana: le')).toBeInTheDocument();
  });

  it('allowsEditingOnWideScreen', async () => {
    responderCom({ 'GET /api/eu': () => json(construtor) });
    simularTela(false);
    renderComProvedores(<Probe />);
    expect(await screen.findByText('Ana: edita')).toBeInTheDocument();
  });
});

describe('useEu sem sessão', () => {
  it('showsUnauthenticatedScreenInLayout', async () => {
    responderCom({ 'GET /api/eu': () => json({ codigo: 'NAO_AUTENTICADO', error: 'x' }, 401) });
    renderComProvedores(<StudioLayout />);
    expect(await screen.findByText('Não reconhecemos seu usuário.')).toBeInTheDocument();
  });
});
