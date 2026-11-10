import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import fixture from '@/fluxo/modelo/agendar-mudanca.fixture.json';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { ComparacaoPage } from './ComparacaoPage';
import type { Diferenca } from './contratos';

const gestor: Eu = {
  id: 'u1',
  nome: 'Bia',
  papel: 'gestor_administradora',
  permissoes: ['skills.ler', 'skills.decidir_atualizacao'],
  administradoraId: 'adm-1',
  dev: false,
};
const soLeitura: Eu = { ...gestor, permissoes: ['skills.ler'] };

const dif = (trecho: string, extra: Partial<Diferenca> = {}): Diferenca => ({
  trecho,
  tipo: 'texto',
  rotulo: trecho,
  situacao: 'so_base',
  obrigatorio: false,
  antes: 'antes',
  depois: 'depois',
  minha: 'antes',
  ...extra,
});

const resumo = dif('texto:resumo', { rotulo: 'Resumo' });
const titulo = dif('texto:titulo', {
  rotulo: 'Título',
  situacao: 'conflito',
  antes: 'Agendar',
  depois: 'Agendar a mudança',
  minha: 'Marcar mudança',
});

const etapa = (tipo: 'confirmar' | 'responder', texto: string) =>
  JSON.stringify({ id: 'e-1', tipo, texto });

const atualizacao = (diferencas: Diferenca[], extra: Record<string, unknown> = {}) => ({
  id: 'at-1',
  camada: { skill: 'agendar-mudanca', tipo: 'administradora', id: 'adm-1' },
  estado: 'pendente',
  cabeca: 3,
  para: { numero: 8, resumo: 'Textos novos' },
  prazoEm: '2026-11-18T12:00:00Z',
  diferencas,
  ...extra,
});

const camadaLida = {
  camada: { skill: 'agendar-mudanca', tipo: 'administradora', id: 'adm-1' },
  documento: {
    textos: {
      titulo: 'Agendar mudança',
      quandoUsar: 'Quando o morador quer marcar a mudança.',
      frasesDeExemplo: [],
      palavrasChave: [],
      temFluxo: true,
      resumo: '',
      visibilidade: 'private',
    },
    fluxo: fixture.fluxo,
    ferramentas: [],
    orientacao: '',
  },
  versao: 'v1',
  temRascunho: false,
  publicada: null,
  ferramentasDaBase: [],
  problemas: [],
};

type Resposta = (req: Request) => Response | Promise<Response>;
const DECISAO = 'POST /api/atualizacoes/at-1/decisao';

function rotas(dados: unknown, eu: Eu = gestor, extras: Record<string, Resposta> = {}) {
  return {
    'GET /api/eu': () => json(eu),
    'GET /api/admin/estado': () => json({ mapeamento: {}, administradoras: [], casos: [] }),
    'GET /api/atualizacoes/at-1': () => json(dados),
    'GET /api/skills/agendar-mudanca/administradoras/adm-1': () => json(camadaLida),
    'GET /api/skills/agendar-mudanca/condominios/vila': () =>
      json({ ...camadaLida, camada: { ...camadaLida.camada, tipo: 'condominio', id: 'vila' } }),
    'GET /api/nexus/recursos': () =>
      json({ account: 'conta', origem: 'nexus', conferido: true, recursos: [] }),
    ...extras,
  };
}

/** Guarda o corpo enviado e responde como o servidor responderia. */
function decisaoQueGuarda(corpos: unknown[], resposta: () => Response) {
  return {
    [DECISAO]: async (req: Request) => {
      corpos.push(await req.json());
      return resposta();
    },
  };
}

const decidida = () => json({ numero: 5, estado: 'aceita' }, 201);

const montar = () =>
  renderComProvedores(<ComparacaoPage />, {
    rota: '/atualizacoes/at-1',
    caminho: '/atualizacoes/:id',
  });

const secao = async (nome: string) => within(await screen.findByRole('region', { name: nome }));

describe('ComparacaoPage', () => {
  it('showsVersionAndDeadline', async () => {
    responderCom(rotas(atualizacao([resumo])));
    montar();
    expect(await screen.findByText('Nova versão 8: Textos novos')).toBeInTheDocument();
    expect(screen.getByText(/Chega sozinha em 18\/11\/2026/)).toBeInTheDocument();
    expect(await screen.findByText('Atualização · Agendar mudança')).toBeInTheDocument();
  });

  it('showsThreeColumnsPerSection', async () => {
    responderCom(rotas(atualizacao([titulo])));
    montar();
    const trecho = await secao('Título');
    expect(trecho.getByRole('group', { name: 'Como era · Superlógica' })).toHaveTextContent(
      'Agendar',
    );
    expect(
      trecho.getByRole('group', { name: 'Como ficou · Superlógica, versão 8' }),
    ).toHaveTextContent('Agendar a mudança');
    expect(trecho.getByRole('group', { name: 'Sua versão · Administradora' })).toHaveTextContent(
      'Marcar mudança',
    );
  });

  it('namesTheCondominiumAndItsAdministratorAsOrigins', async () => {
    const doCondominio = { ...atualizacao([titulo]).camada, tipo: 'condominio', id: 'vila' };
    responderCom(rotas(atualizacao([titulo], { camada: doCondominio })));
    montar();
    const trecho = await secao('Título');
    expect(trecho.getByRole('group', { name: 'Como era · Administradora' })).toBeInTheDocument();
    expect(trecho.getByRole('group', { name: 'Sua versão · Condomínio' })).toBeInTheDocument();
  });

  it('blocksSaveWhileConflictPending', async () => {
    responderCom(rotas(atualizacao([resumo, titulo])));
    const { user } = montar();
    const salvar = await screen.findByRole('button', { name: 'Salvar decisão' });
    expect(salvar).toBeDisabled();
    expect(screen.getByText('Falta decidir 1 trecho em conflito.')).toBeInTheDocument();
    await user.click(
      within(await screen.findByRole('region', { name: 'Título' })).getByRole('button', {
        name: 'Ficar com a minha',
      }),
    );
    expect(salvar).toBeEnabled();
    expect(screen.queryByText(/Falta decidir/)).not.toBeInTheDocument();
  });

  it('sendsMixForConflict', async () => {
    const corpos: unknown[] = [];
    responderCom(rotas(atualizacao([resumo, titulo]), gestor, decisaoQueGuarda(corpos, decidida)));
    const { user } = montar();
    const trecho = await secao('Título');
    await user.click(trecho.getByRole('button', { name: 'Editar a mistura' }));
    const campo = await screen.findByRole('textbox', { name: 'Mistura de Título' });
    expect(campo).toHaveValue('Marcar mudança');
    await user.clear(campo);
    await user.type(campo, 'Marcar a mudança');
    await user.click(screen.getByRole('button', { name: 'Usar esta mistura' }));
    await user.click(screen.getByRole('button', { name: 'Salvar decisão' }));
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toEqual({
      cabeca: 3,
      decisoes: [
        { trecho: 'texto:resumo', escolha: 'aceitar' },
        { trecho: 'texto:titulo', escolha: 'misturar', mistura: 'Marcar a mudança' },
      ],
    });
    expect(await screen.findByText('Atualização decidida')).toBeInTheDocument();
  });

  it('mixesAStepWithItsPanel', async () => {
    const corpos: unknown[] = [];
    const passo = dif('etapa:e-1', {
      tipo: 'etapa',
      rotulo: 'Etapa Confirmar',
      situacao: 'conflito',
      antes: etapa('confirmar', 'Posso agendar?'),
      depois: etapa('confirmar', 'Posso agendar para o dia?'),
      minha: etapa('confirmar', 'Pode marcar?'),
    });
    responderCom(rotas(atualizacao([passo]), gestor, decisaoQueGuarda(corpos, decidida)));
    const { user } = montar();
    const trecho = await secao('Etapa Confirmar');
    expect(trecho.getAllByText('Confirmar').length).toBeGreaterThan(0);
    await user.click(trecho.getByRole('button', { name: 'Editar a mistura' }));
    const campo = await screen.findByRole('textbox', { name: 'Pergunta de confirmação' });
    expect(campo).toHaveValue('Pode marcar?');
    await user.type(campo, ' Para o dia?');
    expect(screen.queryByRole('button', { name: 'Excluir bloco' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Usar esta mistura' }));
    await user.click(screen.getByRole('button', { name: 'Salvar decisão' }));
    await waitFor(() => expect(corpos).toHaveLength(1));
    const [decisao] = (corpos[0] as { decisoes: { mistura: string }[] }).decisoes;
    expect(JSON.parse(decisao?.mistura ?? '')).toMatchObject({
      id: 'e-1',
      tipo: 'confirmar',
      texto: 'Pode marcar? Para o dia?',
    });
  });

  it('locksMandatorySection', async () => {
    const corpos: unknown[] = [];
    const obrigatoria = dif('texto:resumo', {
      rotulo: 'Resumo',
      obrigatorio: true,
      situacao: 'conflito',
    });
    responderCom(rotas(atualizacao([obrigatoria]), gestor, decisaoQueGuarda(corpos, decidida)));
    const { user } = montar();
    const recusar = await screen.findByRole('button', { name: 'Recusar tudo' });
    const trecho = await secao('Resumo');
    expect(trecho.getByText('Obrigatório')).toBeInTheDocument();
    expect(trecho.getByRole('button', { name: 'Ficar com a minha' })).toBeDisabled();
    expect(trecho.getByRole('button', { name: 'Aceitar a nova' })).toBeDisabled();
    expect(trecho.getByRole('button', { name: 'Aceitar a nova' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );
    await user.click(recusar);
    await user.click(screen.getByRole('button', { name: 'Salvar decisão' }));
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toMatchObject({ decisoes: [{ trecho: 'texto:resumo', escolha: 'aceitar' }] });
  });

  it('warnsWhenBaseRemovedWhatThePersonEdited', async () => {
    const removida = dif('texto:resumo', {
      rotulo: 'Resumo',
      situacao: 'conflito',
      depois: null,
      minha: 'meu resumo',
    });
    responderCom(rotas(atualizacao([removida])));
    montar();
    const trecho = await secao('Resumo');
    expect(
      trecho.getByText(/A Superlógica removeu este trecho, mas você tinha alterado/),
    ).toBeInTheDocument();
    expect(trecho.getByText('Removido na versão nova')).toBeInTheDocument();
    expect(trecho.getByRole('button', { name: 'Aceitar a remoção' })).toBeInTheDocument();
    expect(trecho.queryByRole('button', { name: 'Editar a mistura' })).not.toBeInTheDocument();
    expect(await screen.findByRole('button', { name: 'Salvar decisão' })).toBeDisabled();
  });

  it('warnsWhenThePersonRemovedWhatTheBaseChanged', async () => {
    const apagada = dif('texto:resumo', { rotulo: 'Resumo', situacao: 'conflito', minha: null });
    responderCom(rotas(atualizacao([apagada])));
    montar();
    const trecho = await secao('Resumo');
    expect(
      trecho.getByText(/Você removeu este trecho e ele foi alterado na versão nova/),
    ).toBeInTheDocument();
    expect(trecho.getByRole('button', { name: 'Manter removido' })).toBeInTheDocument();
  });

  it('showsServerProblemsOnSection', async () => {
    const passo = dif('etapa:e-1', {
      tipo: 'etapa',
      rotulo: 'Etapa Gravar',
      antes: etapa('confirmar', 'a'),
      depois: etapa('confirmar', 'b'),
      minha: etapa('confirmar', 'a'),
    });
    const problema = {
      blocoId: 'e-1',
      codigo: 'GRAVAR_SEM_CONFIRMAR',
      nivel: 'erro',
      mensagem: 'Gravar sem confirmar antes não pode.',
    };
    const recusa = () =>
      json({ codigo: 'FLUXO_INVALIDO', mensagem: 'fluxo inválido', problemas: [problema] }, 422);
    responderCom(rotas(atualizacao([passo]), gestor, decisaoQueGuarda([], recusa)));
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Salvar decisão' }));
    const trecho = await secao('Etapa Gravar');
    expect(await trecho.findByText('Gravar sem confirmar antes não pode.')).toBeInTheDocument();
  });

  it('showsReloadOnStaleVersion', async () => {
    const velha = () => json({ codigo: 'VERSAO_DESATUALIZADA', mensagem: 'a camada andou' }, 409);
    responderCom(rotas(atualizacao([resumo]), gestor, decisaoQueGuarda([], velha)));
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Salvar decisão' }));
    expect(
      await screen.findByText('Esta versão mudou. Recarregue para decidir de novo.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Recarregar' })).toBeInTheDocument();
  });

  it('decidesTenSectionsWithAcceptAllInOneClick', async () => {
    const corpos: unknown[] = [];
    const dez = Array.from({ length: 10 }, (_, i) =>
      dif(`texto:campo${i}`, { rotulo: `Campo ${i}` }),
    );
    responderCom(rotas(atualizacao(dez), gestor, decisaoQueGuarda(corpos, decidida)));
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Salvar decisão' }));
    await waitFor(() => expect(corpos).toHaveLength(1));
    const { decisoes } = corpos[0] as { decisoes: { escolha: string }[] };
    expect(decisoes).toHaveLength(10);
    expect(decisoes.every((d) => d.escolha === 'aceitar')).toBe(true);
  });

  it('hidesDecisionWithoutPermission', async () => {
    responderCom(rotas(atualizacao([resumo]), soLeitura));
    montar();
    expect(await screen.findByText(/decidir exige permissão/)).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Salvar decisão' })).not.toBeInTheDocument();
  });
});
