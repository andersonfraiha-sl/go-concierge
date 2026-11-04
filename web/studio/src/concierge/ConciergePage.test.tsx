import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { ConciergePage, ROTULOS_DE_TOM, TONS } from './ConciergePage';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
  administradoraId: null,
  dev: false,
};
const gestor: Eu = { ...construtor, papel: 'gestor', administradoraId: 'adm-1' };

function rotas(
  eu: Eu,
  persona = 'Você é gentil.',
  aoSalvar: (corpo: unknown) => void = () => {},
  extra: Record<string, () => Response | Promise<Response>> = {},
) {
  return {
    'GET /api/eu': () => json(eu),
    'GET /api/skills': () => json({ assistente: 'concierge', skills: [] }),
    'GET /api/agente': () =>
      json({
        agente: {
          caminho: 'concierge',
          nome: 'concierge',
          descricao: '',
          persona,
          arquivos: {},
          herdados: {},
          problemas: [],
        },
        recursos: [],
      }),
    'PUT /api/arquivo': async (req: Request) => {
      aoSalvar(await req.json());
      return json({ salvo: 'persona.md' });
    },
    ...extra,
  };
}

describe('ConciergePage', () => {
  it('savesPersonaOneSecondAfterTyping', async () => {
    const corpos: unknown[] = [];
    responderCom(rotas(construtor, 'Você é gentil.', (c) => corpos.push(c)));
    const { user } = renderComProvedores(<ConciergePage />);
    const campo = await screen.findByDisplayValue('Você é gentil.');
    await user.type(campo, ' Sempre.');
    expect(corpos).toHaveLength(0);
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos[0]).toEqual({
      agente: 'concierge',
      caminho: 'persona.md',
      conteudo: 'Você é gentil. Sempre.',
    });
  });

  it('gestorReadsPersona', async () => {
    responderCom(rotas(gestor));
    renderComProvedores(<ConciergePage />);
    expect(await screen.findByDisplayValue('Você é gentil.')).toHaveAttribute('readonly');
    expect(
      screen.getByText('A voz do concierge em todas as respostas ao morador.'),
    ).toBeInTheDocument();
  });

  it('warnsWhenPersonaIsEmpty', async () => {
    responderCom(rotas(construtor, ''));
    renderComProvedores(<ConciergePage />);
    expect(
      await screen.findByText('Sem persona, o concierge responde sem um jeito próprio de falar.'),
    ).toBeInTheDocument();
  });

  it('fillsPersonaFromTemplate', async () => {
    const corpos: unknown[] = [];
    const modelo = 'Fale como um vizinho prestativo e gentil, sempre pelo primeiro nome.';
    responderCom(
      rotas(construtor, '', (c) => corpos.push(c), {
        'GET /api/arquivo/modelo': () => json({ caminho: 'persona.md', conteudo: modelo }),
      }),
    );
    const { user } = renderComProvedores(<ConciergePage />);
    await user.click(await screen.findByRole('button', { name: 'Usar o modelo' }));
    await screen.findByDisplayValue(modelo);
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos[0]).toEqual({ agente: 'concierge', caminho: 'persona.md', conteudo: modelo });
  });

  it('gestorSeesNoTemplateButton', async () => {
    responderCom(rotas(gestor, ''));
    renderComProvedores(<ConciergePage />);
    await screen.findByText('Sem persona, o concierge responde sem um jeito próprio de falar.');
    expect(screen.queryByRole('button', { name: 'Usar o modelo' })).not.toBeInTheDocument();
  });

  it.each(Object.entries(TONS) as Array<[keyof typeof TONS, string]>)(
    'pickingToneReplacesToneSentenceOnly(%s)',
    async (tom, frase) => {
      const outraFrase = tom === 'acolhedor' ? TONS.direto : TONS.acolhedor;
      const personaInicial = `Saudação gentil ao morador. ${outraFrase} Encerre desejando um bom dia.`;
      const corpos: unknown[] = [];
      responderCom(rotas(construtor, personaInicial, (c) => corpos.push(c)));
      const { user } = renderComProvedores(<ConciergePage />);
      await screen.findByDisplayValue(personaInicial);

      await user.click(await screen.findByRole('button', { name: ROTULOS_DE_TOM[tom] }));

      const esperado = `Saudação gentil ao morador. ${frase} Encerre desejando um bom dia.`;
      await screen.findByDisplayValue(esperado);
      await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
      expect(corpos[0]).toEqual({
        agente: 'concierge',
        caminho: 'persona.md',
        conteudo: esperado,
      });
    },
  );

  it('gestorSeesNoToneButtons', async () => {
    responderCom(rotas(gestor));
    renderComProvedores(<ConciergePage />);
    await screen.findByDisplayValue('Você é gentil.');
    for (const rotulo of Object.values(ROTULOS_DE_TOM)) {
      expect(screen.queryByRole('button', { name: rotulo })).not.toBeInTheDocument();
    }
  });

  it('showsAllFiveConciergeTabs', async () => {
    responderCom(rotas(construtor));
    renderComProvedores(<ConciergePage />);
    await screen.findByDisplayValue('Você é gentil.');
    for (const rotulo of [
      'Persona',
      'Boas-vindas',
      'Regras gerais',
      'Base de conhecimento comum',
      'Transbordo',
    ]) {
      expect(screen.getByRole('tab', { name: rotulo })).toBeInTheDocument();
    }
  });

  it('opensTabFromQuery', async () => {
    responderCom(rotas(construtor));
    renderComProvedores(<ConciergePage />, { rota: '/?aba=regras' });
    expect(await screen.findByRole('heading', { name: 'Regras gerais' })).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Texto da persona' })).not.toBeInTheDocument();
  });
});
