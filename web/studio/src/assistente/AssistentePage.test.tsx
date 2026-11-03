import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { AssistentePage } from './AssistentePage';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
  administradoraId: null,
  dev: false,
};
const gestor: Eu = { ...construtor, papel: 'gestor', administradoraId: 'adm-1' };

function rotas(eu: Eu, aoSalvar: (corpo: unknown) => void = () => {}) {
  return {
    'GET /api/eu': () => json(eu),
    'GET /api/skills': () => json({ assistente: 'concierge', skills: [] }),
    'GET /api/agente': () =>
      json({
        agente: {
          caminho: 'concierge',
          nome: 'concierge',
          descricao: '',
          persona: 'Você é gentil.',
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
  };
}

describe('AssistentePage', () => {
  it('savesPersonaOneSecondAfterTyping', async () => {
    const corpos: unknown[] = [];
    responderCom(rotas(construtor, (c) => corpos.push(c)));
    const { user } = renderComProvedores(<AssistentePage />);
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
    renderComProvedores(<AssistentePage />);
    expect(await screen.findByDisplayValue('Você é gentil.')).toHaveAttribute('readonly');
    expect(
      screen.getByText('A voz da assistente em todas as respostas ao morador.'),
    ).toBeInTheDocument();
  });
});
