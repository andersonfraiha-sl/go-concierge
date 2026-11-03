import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { pode, podeNoCatalogo, usePodeEditar, type Eu } from './eu';

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
const gestorAvancado: Eu = { ...gestor, papel: 'gestor_avancado', permissoes: [...gestor.permissoes, 'studio.avancado'] };

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
  return <span>{usePodeEditar('skills.editar_fluxo') ? 'edita' : 'le'}</span>;
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
    // espera o /api/eu chegar: antes dele o hook também devolve "le"
    await vi.waitFor(() => expect(fetch).toHaveBeenCalled());
    await new Promise((r) => setTimeout(r, 50));
    expect(screen.getByText('le')).toBeInTheDocument();
  });

  it('allowsEditingOnWideScreen', async () => {
    responderCom({ 'GET /api/eu': () => json(construtor) });
    simularTela(false);
    renderComProvedores(<Probe />);
    expect(await screen.findByText('edita')).toBeInTheDocument();
  });
});
