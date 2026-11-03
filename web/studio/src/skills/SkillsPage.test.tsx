import { screen, waitFor, within } from '@testing-library/react';
import { useLocation } from 'react-router';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { SkillsPage } from './SkillsPage';

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos', 'skills.criar', 'skills.excluir'],
  administradoraId: null,
  dev: false,
};
const gestor: Eu = {
  ...construtor,
  papel: 'gestor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
};

const mudanca = {
  nome: 'agendar-mudanca',
  id: 'agendar-mudanca',
  titulo: 'Agendar mudança',
  descricao: 'Reserva o elevador para a mudança.',
  escrita: true,
  tipo: 'fluxo',
  recursos: ['reservar', 'listar'],
  caminho: 'skills/agendar-mudanca',
};
const faq: typeof mudanca = {
  ...mudanca,
  nome: 'faq',
  id: 'faq',
  titulo: 'Perguntas frequentes',
  escrita: false,
  recursos: ['buscar'],
};

const lista = (skills: (typeof mudanca)[]) => json({ assistente: 'concierge', skills });

function Local() {
  return <output aria-label="rota">{useLocation().pathname}</output>;
}

function montar() {
  return renderComProvedores(
    <>
      <SkillsPage />
      <Local />
    </>,
    { rota: '/skills' },
  );
}

describe('SkillsPage', () => {
  it('listsSkillCards', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([mudanca, faq]),
    });
    montar();
    const cartao = await screen.findByRole('article', { name: 'Agendar mudança' });
    expect(within(cartao).getByText('Reserva o elevador para a mudança.')).toBeInTheDocument();
    expect(within(cartao).getByText('Grava no sistema')).toBeInTheDocument();
    expect(within(cartao).getByText('2 ferramentas')).toBeInTheDocument();
    const outro = screen.getByRole('article', { name: 'Perguntas frequentes' });
    expect(within(outro).queryByText('Grava no sistema')).not.toBeInTheDocument();
    expect(within(outro).getByText('1 ferramenta')).toBeInTheDocument();
  });

  it('showsEmptyState', async () => {
    responderCom({ 'GET /api/eu': () => json(construtor), 'GET /api/skills': () => lista([]) });
    montar();
    expect(
      await screen.findByText(
        'Nenhuma skill ainda. Uma skill é algo que a assistente sabe fazer pelo morador, como reservar uma área comum.',
      ),
    ).toBeInTheDocument();
  });

  it('offersRetryWhenListFails', async () => {
    let falhas = 1;
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () =>
        falhas-- > 0 ? json({ codigo: 'X', mensagem: 'Servidor caiu.' }, 500) : lista([faq]),
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Tentar de novo' }));
    expect(
      await screen.findByRole('article', { name: 'Perguntas frequentes' }),
    ).toBeInTheDocument();
  });

  it('createsSkillAndOpensEditor', async () => {
    const corpos: unknown[] = [];
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'POST /api/skills': async (req) => {
        corpos.push(await req.json());
        return json({ nome: 'reserva-do-salao' }, 201);
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Nova skill' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Reserva do Salão');
    expect(screen.getByText('Identificador: reserva-do-salao')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Criar' }));
    expect(await screen.findByText('Skill criada.')).toBeInTheDocument();
    expect(corpos).toEqual([{ nome: 'reserva-do-salao', titulo: 'Reserva do Salão' }]);
    await waitFor(() =>
      expect(screen.getByLabelText('rota')).toHaveTextContent('/skills/reserva-do-salao'),
    );
  });

  it('duplicatesSkillStartingFromCopyName', async () => {
    const corpos: unknown[] = [];
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'POST /api/skills/faq/duplicar': async (req) => {
        corpos.push(await req.json());
        return json({ nome: 'copia-de-perguntas-frequentes' }, 201);
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Duplicar Perguntas frequentes' }));
    expect(screen.getByRole('textbox', { name: 'Nome' })).toHaveValue(
      'Cópia de Perguntas frequentes',
    );
    await user.click(screen.getByRole('button', { name: 'Duplicar' }));
    expect(await screen.findByText('Skill duplicada.')).toBeInTheDocument();
    expect(corpos).toEqual([
      { nome: 'copia-de-perguntas-frequentes', titulo: 'Cópia de Perguntas frequentes' },
    ]);
  });

  it('showsNameErrorFromServer', async () => {
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'POST /api/skills': () =>
        json({ codigo: 'SKILL_JA_EXISTE', mensagem: 'Já existe uma skill com esse nome.' }, 409),
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Nova skill' }));
    await user.type(screen.getByRole('textbox', { name: 'Nome' }), 'Faq');
    await user.click(screen.getByRole('button', { name: 'Criar' }));
    expect(await screen.findByText('Já existe uma skill com esse nome.')).toBeInTheDocument();
    expect(screen.getByLabelText('rota')).toHaveTextContent('/skills');
  });

  it('deletesAfterConfirmation', async () => {
    const chamadas: string[] = [];
    responderCom({
      'GET /api/eu': () => json(construtor),
      'GET /api/skills': () => lista([faq]),
      'DELETE /api/skills/faq': () => {
        chamadas.push('DELETE');
        return new Response(null, { status: 204 });
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Excluir Perguntas frequentes' }));
    expect(
      screen.getByText('Excluir "Perguntas frequentes"? O fluxo e as regras somem junto.'),
    ).toBeInTheDocument();
    expect(chamadas).toEqual([]);
    await user.click(screen.getByRole('button', { name: 'Excluir' }));
    expect(await screen.findByText('Skill excluída.')).toBeInTheDocument();
    expect(chamadas).toEqual(['DELETE']);
  });

  it('gestorSeesNoStructureButtons', async () => {
    responderCom({ 'GET /api/eu': () => json(gestor), 'GET /api/skills': () => lista([faq]) });
    montar();
    await screen.findByRole('article', { name: 'Perguntas frequentes' });
    expect(screen.getByRole('button', { name: 'Abrir Perguntas frequentes' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Nova skill' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Duplicar/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Excluir/ })).not.toBeInTheDocument();
  });
});

describe('SkillsPage em tela estreita', () => {
  it('narrowScreenHidesStructureButtons', async () => {
    const original = window.matchMedia;
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: (query: string) => ({ ...original(query), matches: true }),
    });
    try {
      responderCom({
        'GET /api/eu': () => json(construtor),
        'GET /api/skills': () => lista([faq]),
      });
      montar();
      await screen.findByRole('article', { name: 'Perguntas frequentes' });
      expect(
        screen.getByRole('button', { name: 'Abrir Perguntas frequentes' }),
      ).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Nova skill' })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Duplicar/ })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Excluir/ })).not.toBeInTheDocument();
    } finally {
      Object.defineProperty(window, 'matchMedia', {
        configurable: true,
        writable: true,
        value: original,
      });
    }
  });
});
