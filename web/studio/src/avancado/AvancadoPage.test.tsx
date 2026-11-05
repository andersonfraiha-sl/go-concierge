import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { AvancadoPage } from './AvancadoPage';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'studio.avancado'],
  administradoraId: null,
  dev: false,
};
const gestorAvancado: Eu = { ...construtor, papel: 'gestor_avancado', administradoraId: 'adm-1' };

const arvore = {
  agentes: [
    {
      caminho: 'concierge',
      nome: 'concierge',
      tipo: 'agente',
      descricao: '',
      caso: false,
      erros: 1,
      avisos: 2,
      subagentes: [],
    },
  ],
};

const agente = {
  agente: {
    caminho: 'concierge',
    nome: 'concierge',
    descricao: '',
    persona: 'Voz.',
    tipo: 'orquestrador',
    tipo_declarado: true,
    arquivos: { rules: [{ caminho: 'rules/politicas.md', bytes: 12 }] },
    herdados: {},
    problemas: [{ nivel: 'erro', onde: 'rules/politicas.md', texto: 'Está vazio.' }],
  },
  recursos: [],
};

const orquestracao = {
  agente: 'concierge',
  arquivo: { modo: 'router', bind_confidence: 0.8, max_itens: 5, max_iteracoes: 8 },
  efetivo: { modo: 'router', bind_confidence: 0.8, max_itens: 5, max_iteracoes: 8 },
  ambiente: '',
};

type Rota = (req: Request) => Response | Promise<Response>;

function rotas(eu: Eu, extras: Record<string, Rota> = {}): Record<string, Rota> {
  return {
    'GET /api/eu': () => json(eu),
    'GET /api/skills': () => json({ assistente: 'concierge', skills: [] }),
    'GET /api/agentes': () => json(arvore),
    'GET /api/agente': () => json(agente),
    'GET /api/arquivo': () => json({ caminho: 'rules/politicas.md', conteudo: 'Regra antiga' }),
    'GET /api/projecao': () => json({ projecao: 'TEXTO MONTADO', roteador: 'TEXTO DO ROTEADOR' }),
    'GET /api/admin/orquestracao': () => json(orquestracao),
    ...extras,
  };
}

function gravadorDe(chave: string, corpos: unknown[], resposta: unknown): Record<string, Rota> {
  return {
    [chave]: async (req) => {
      corpos.push(await req.json());
      return json(resposta);
    },
  };
}

describe('AvancadoPage', () => {
  it('opensAndSavesFile', async () => {
    const corpos: unknown[] = [];
    responderCom(
      rotas(construtor, gravadorDe('PUT /api/arquivo', corpos, { salvo: 'rules/politicas.md' })),
    );
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=arquivos' });
    await user.click(await screen.findByRole('button', { name: 'politicas.md' }));
    const editor = await screen.findByDisplayValue('Regra antiga');
    await user.clear(editor);
    await user.type(editor, 'Regra nova');
    expect(corpos).toHaveLength(0);
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toEqual({
      agente: 'concierge',
      caminho: 'rules/politicas.md',
      conteudo: 'Regra nova',
    });
  });

  it('showsProjectionReadOnly', async () => {
    responderCom(rotas(construtor));
    renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=projecao' });
    expect(await screen.findByText('TEXTO MONTADO')).toBeInTheDocument();
    expect(screen.getByText('TEXTO DO ROTEADOR')).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });

  it('rejectsOutOfRangeOrchestration', async () => {
    const corpos: unknown[] = [];
    responderCom(
      rotas(construtor, gravadorDe('PUT /api/admin/orquestracao', corpos, orquestracao)),
    );
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=orquestracao' });
    const campo = await screen.findByLabelText('Itens por plano');
    await user.clear(campo);
    await user.type(campo, '11');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    expect(await screen.findByText('Itens por plano deve ficar entre 1 e 10.')).toBeInTheDocument();
    expect(corpos).toHaveLength(0);
  });

  it('savesValidOrchestration', async () => {
    const corpos: unknown[] = [];
    responderCom(
      rotas(construtor, gravadorDe('PUT /api/admin/orquestracao', corpos, orquestracao)),
    );
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=orquestracao' });
    const campo = await screen.findByLabelText('Itens por plano');
    await user.clear(campo);
    await user.type(campo, '7');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toEqual({
      modo: 'router',
      bind_confidence: 0.8,
      max_itens: 7,
      max_iteracoes: 8,
    });
  });

  it('showsEnvironmentOverride', async () => {
    responderCom(
      rotas(construtor, {
        'GET /api/admin/orquestracao': () => json({ ...orquestracao, ambiente: 'react' }),
      }),
    );
    renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=orquestracao' });
    expect(await screen.findByText(/forçando o modo react/)).toBeInTheDocument();
  });

  it('linksExportOfChosenItem', async () => {
    responderCom(rotas(construtor));
    renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=importar' });
    const link = await screen.findByRole('link', { name: 'Exportar zip' });
    expect(link).toHaveAttribute('href', '/api/agentes/exportar?agente=concierge');
  });

  it('importsZipToChosenPath', async () => {
    const urls: string[] = [];
    responderCom(
      rotas(construtor, {
        'POST /api/agentes/importar': (req) => {
          urls.push(req.url);
          return json({ instalado: 'concierge/agentes/nova', arquivos: ['agent.yaml'] });
        },
      }),
    );
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=importar' });
    await user.type(await screen.findByLabelText('Instalar em'), 'concierge/agentes/nova');
    await user.upload(
      screen.getByLabelText('Arquivo zip'),
      new File(['zip'], 'nova.zip', { type: 'application/zip' }),
    );
    await user.click(screen.getByRole('button', { name: 'Importar' }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: 'Importar' }),
    );
    await waitFor(() => expect(urls).toHaveLength(1));
    expect(new URL(urls[0] ?? '').searchParams.get('caminho')).toBe('concierge/agentes/nova');
    expect(
      await screen.findByText('Importado em concierge/agentes/nova: 1 arquivos'),
    ).toBeInTheDocument();
  });

  it('showsErrorWhenImportFails', async () => {
    responderCom(
      rotas(construtor, {
        'POST /api/agentes/importar': () =>
          json({ codigo: 'ZIP_INVALIDO', mensagem: 'Zip corrompido.' }, 400),
      }),
    );
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=importar' });
    await user.type(await screen.findByLabelText('Instalar em'), 'concierge');
    await user.upload(
      screen.getByLabelText('Arquivo zip'),
      new File(['zip'], 'a.zip', { type: 'application/zip' }),
    );
    await user.click(screen.getByRole('button', { name: 'Importar' }));
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: 'Importar' }),
    );
    expect(await screen.findByText('Zip corrompido.')).toBeInTheDocument();
  });

  it('deletesFileAfterConfirmation', async () => {
    const excluidos: string[] = [];
    responderCom(
      rotas(construtor, {
        'DELETE /api/arquivo': (req) => {
          excluidos.push(new URL(req.url).searchParams.get('caminho') ?? '');
          return json({ excluido: 'rules/politicas.md' });
        },
      }),
    );
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=arquivos' });
    await user.click(await screen.findByRole('button', { name: 'politicas.md' }));
    await user.click(await screen.findByRole('button', { name: 'Excluir' }));
    expect(excluidos).toHaveLength(0);
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('button', { name: 'Excluir' }),
    );
    await waitFor(() => expect(excluidos).toEqual(['rules/politicas.md']));
  });

  it('gestorWithGrantSeesOrchestrationReadOnlyAndNoImport', async () => {
    responderCom(rotas(gestorAvancado));
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=orquestracao' });
    expect(await screen.findByLabelText('Itens por plano')).toHaveAttribute('readonly');
    expect(screen.queryByRole('button', { name: 'Salvar' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('tab', { name: 'Importar e exportar' }));
    expect(await screen.findByRole('link', { name: 'Exportar zip' })).toBeInTheDocument();
    expect(screen.queryByLabelText('Instalar em')).not.toBeInTheDocument();
  });

  it('showsEmptyStateWithoutItems', async () => {
    responderCom(rotas(construtor, { 'GET /api/agentes': () => json({ agentes: [] }) }));
    renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=arquivos' });
    expect(
      await screen.findByText('Ainda não há concierge nem skills para mostrar.'),
    ).toBeInTheDocument();
  });

  it('renamesProjectionToAssembledPrompt', async () => {
    responderCom(rotas(construtor));
    renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=arquivos' });
    expect(await screen.findByRole('tab', { name: 'Prompt montado' })).toBeInTheDocument();
    expect(screen.queryByRole('tab', { name: 'Projeção' })).not.toBeInTheDocument();
  });

  it('warnsThatChangesAffectAllCondominios', async () => {
    responderCom(rotas(construtor));
    renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=arquivos' });
    expect(
      await screen.findByText('Mudanças aqui afetam todos os condomínios'),
    ).toBeInTheDocument();
  });

  it('showsLineNumbers', async () => {
    responderCom(rotas(construtor));
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=arquivos' });
    await user.click(await screen.findByRole('button', { name: 'politicas.md' }));
    await screen.findByDisplayValue('Regra antiga');
    expect(screen.getByText('1')).toBeInTheDocument();
  });

  it('keepsLineNumbersAlignedWithAVeryLongLine', async () => {
    const linhaMuitoLonga = 'x'.repeat(400);
    const conteudo = [linhaMuitoLonga, 'linha 2', 'linha 3', 'linha 4', 'linha 5'].join('\n');
    responderCom(
      rotas(construtor, {
        'GET /api/arquivo': () => json({ caminho: 'rules/politicas.md', conteudo }),
      }),
    );
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=arquivos' });
    await user.click(await screen.findByRole('button', { name: 'politicas.md' }));
    await screen.findByLabelText('Conteúdo de rules/politicas.md');
    const numerosExibidos = (
      await screen.findByText(
        (_, elemento) => elemento?.tagName === 'PRE' && /^1\n/.test(elemento.textContent ?? ''),
      )
    ).textContent;
    expect(numerosExibidos).toBe('1\n2\n3\n4\n5');
  });

  it('gestorWithGrantReadsOnly', async () => {
    responderCom(rotas(gestorAvancado));
    const { user } = renderComProvedores(<AvancadoPage />, { rota: '/avancado?aba=arquivos' });
    await user.click(await screen.findByRole('button', { name: 'politicas.md' }));
    expect(await screen.findByDisplayValue('Regra antiga')).toHaveAttribute('readonly');
    expect(screen.queryByRole('button', { name: 'Salvar' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Novo arquivo' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Excluir' })).not.toBeInTheDocument();
  });
});
