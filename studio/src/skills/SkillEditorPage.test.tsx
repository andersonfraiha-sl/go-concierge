import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { SkillEditorPage } from './SkillEditorPage';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
  administradoraId: null,
  dev: false,
};
const gestor: Eu = { ...construtor, papel: 'gestor', administradoraId: 'adm-1' };

const lidos = (temFluxo = true) => ({
  caminho: 'skills/agendar-mudanca',
  versao: 'v1',
  textos: {
    titulo: 'Agendar mudança',
    quandoUsar: 'Quando o morador quer mudar.',
    frasesDeExemplo: ['Quero agendar uma mudança'],
    palavrasChave: ['mudança'],
    temFluxo,
  },
});

const montar = (rota = '/skills/agendar-mudanca') =>
  renderComProvedores(<SkillEditorPage />, { rota, caminho: '/skills/:nome' });

/** Rotas do editor; cada PUT dos textos devolve a próxima versão e guarda o corpo recebido. */
function rotasDoEditor(eu: Eu, corpos: unknown[], textos = lidos()) {
  return {
    'GET /api/eu': () => json(eu),
    'GET /api/skills/agendar-mudanca': () => json(textos),
    'PUT /api/skills/agendar-mudanca/textos': async (req: Request) => {
      corpos.push(await req.json());
      return json({ versao: 'v2' });
    },
  };
}

describe('SkillEditorPage', () => {
  it('savesQuandoUsarOneSecondAfterTyping', async () => {
    const corpos: unknown[] = [];
    responderCom(rotasDoEditor(construtor, corpos));
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Quando usar' });
    await waitFor(() => expect(campo).toBeEnabled());
    await user.type(campo, ' Sempre.');
    expect(corpos).toEqual([]);
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos).toEqual([
      { versao: 'v1', textos: { ...lidos().textos, quandoUsar: 'Quando o morador quer mudar. Sempre.' } },
    ]);
  });

  it('renamesFromTitle', async () => {
    const corpos: unknown[] = [];
    responderCom(rotasDoEditor(construtor, corpos));
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Editar Nome da skill' }));
    const campo = screen.getByRole('textbox', { name: 'Nome da skill' });
    await user.clear(campo);
    await user.type(campo, 'Mudança de unidade{Enter}');
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos).toEqual([{ versao: 'v1', textos: { ...lidos().textos, titulo: 'Mudança de unidade' } }]);
    expect(await screen.findByText(/Salvo às/)).toBeInTheDocument();
  });

  it('addsKeywordAndExamplePhrase', async () => {
    const corpos: unknown[] = [];
    responderCom(rotasDoEditor(construtor, corpos));
    const { user } = montar();
    await user.type(await screen.findByRole('textbox', { name: 'Nova palavra-chave' }), 'elevador{Enter}');
    await user.click(screen.getByRole('button', { name: 'Adicionar frase' }));
    await user.type(screen.getByRole('textbox', { name: 'Frase de exemplo 2' }), 'Preciso do elevador');
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos).toEqual([
      {
        versao: 'v1',
        textos: {
          ...lidos().textos,
          palavrasChave: ['mudança', 'elevador'],
          frasesDeExemplo: ['Quero agendar uma mudança', 'Preciso do elevador'],
        },
      },
    ]);
  });

  it('hidesExamplesWithoutFlow', async () => {
    responderCom(rotasDoEditor(construtor, [], lidos(false)));
    montar();
    await screen.findByRole('textbox', { name: 'Quando usar' });
    expect(screen.queryByText('Palavras-chave')).not.toBeInTheDocument();
    expect(screen.queryByText('Frases de exemplo')).not.toBeInTheDocument();
  });

  it('gestorReadsOnly', async () => {
    responderCom(rotasDoEditor(gestor, []));
    montar();
    const campo = await screen.findByRole('textbox', { name: 'Quando usar' });
    expect(campo).toBeDisabled();
    expect(screen.getByRole('textbox', { name: 'Frase de exemplo 1' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Adicionar frase' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Editar Nome da skill' })).not.toBeInTheDocument();
  });

  it('opensTabFromQuery', async () => {
    responderCom(rotasDoEditor(construtor, []));
    montar('/skills/agendar-mudanca?aba=regras');
    expect(await screen.findByRole('tab', { name: 'Regras', selected: true })).toBeInTheDocument();
    expect(screen.getAllByRole('tab').map((aba) => aba.textContent)).toEqual([
      'Quando usar',
      'Fluxo',
      'Regras',
      'Base de conhecimento',
      'Ferramentas',
    ]);
  });
});
