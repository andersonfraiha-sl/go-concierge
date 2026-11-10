import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import type { Camada } from './contratos';
import { HistoricoDaCamada } from './HistoricoDaCamada';

const camada: Camada = { skill: 'agendar-mudanca', tipo: 'administradora', id: 'adm-1' };
const BASE = '/api/skills/agendar-mudanca/administradoras/adm-1';

const gestor: Eu = {
  id: 'u1',
  nome: 'Bia',
  papel: 'gestor_administradora',
  permissoes: ['skills.ler', 'skills.editar_textos'],
  administradoraId: 'adm-1',
  dev: false,
};
const soLeitura: Eu = { ...gestor, permissoes: ['skills.ler'] };

const versoes = [
  {
    numero: 3,
    origem: 'decisao',
    autor: 'Bia',
    resumo: '',
    motivo: '',
    criadoEm: '2026-11-04T12:00:00Z',
    decisao: { para: 8, aceitos: 3, mantidos: 1, misturados: 2 },
  },
  {
    numero: 2,
    origem: 'obrigatorio',
    autor: 'sistema',
    resumo: 'Troca da agenda',
    motivo: 'A ferramenta mudou de nome.',
    criadoEm: '2026-11-03T12:00:00Z',
    decisao: null,
  },
  {
    numero: 1,
    origem: 'publicacao',
    autor: 'Ana',
    resumo: 'Primeira versão',
    motivo: '',
    criadoEm: '2026-11-02T12:00:00Z',
    decisao: null,
  },
];

const rotasBase = (eu: Eu) => ({
  'GET /api/eu': () => json(eu),
  [`GET ${BASE}/historico`]: () => json({ versoes }),
});

function montar() {
  const onVoltou = vi.fn();
  const tela = renderComProvedores(<HistoricoDaCamada camada={camada} onVoltou={onVoltou} />);
  return { onVoltou, ...tela };
}

const versao = async (numero: number) =>
  within(await screen.findByRole('listitem', { name: `Versão ${numero}` }));

const voltarPara = 'Voltar para esta versão';

describe('HistoricoDaCamada', () => {
  it('listsVersionsWithDecisionCounts', async () => {
    responderCom(rotasBase(gestor));
    montar();
    const decidida = await versao(3);
    expect(decidida.getByText(/Decisão sobre uma atualização/)).toBeInTheDocument();
    expect(
      decidida.getByText('Sobre a versão 8: aceitou 3, manteve 1, misturou 2'),
    ).toBeInTheDocument();
    const obrigatoria = await versao(2);
    expect(obrigatoria.getByText('Troca da agenda')).toBeInTheDocument();
    expect(obrigatoria.getByText('Motivo: A ferramenta mudou de nome.')).toBeInTheDocument();
    expect(obrigatoria.getByText(/03\/11\/2026 · sistema · Troca obrigatória/)).toBeInTheDocument();
  });

  it('restoresVersionAfterConfirm', async () => {
    const corpos: unknown[] = [];
    responderCom({
      ...rotasBase(gestor),
      [`POST ${BASE}/voltar`]: async (req: Request) => {
        corpos.push(await req.json());
        return json({ numero: 4 }, 201);
      },
    });
    const { user, onVoltou } = montar();
    const antiga = await versao(1);
    await user.click(await antiga.findByRole('button', { name: voltarPara }));
    expect(screen.getByText('Voltar para a versão 1?')).toBeInTheDocument();
    expect(screen.getByText('Uma versão nova é criada com o conteúdo dela.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Voltar' }));
    await waitFor(() => expect(onVoltou).toHaveBeenCalledTimes(1));
    expect(corpos).toEqual([{ numero: 1 }]);
    expect(
      await screen.findByText('Versão 4 criada com o conteúdo da versão 1.'),
    ).toBeInTheDocument();
  });

  it('offersNoRestoreForTheCurrentVersion', async () => {
    responderCom(rotasBase(gestor));
    montar();
    const atual = await versao(3);
    const antiga = await versao(1);
    await antiga.findByRole('button', { name: voltarPara });
    expect(atual.queryByRole('button', { name: voltarPara })).not.toBeInTheDocument();
  });

  it('hidesRestoreWithoutPermission', async () => {
    const quemSou = vi.fn(() => json(soLeitura));
    responderCom({ ...rotasBase(soLeitura), 'GET /api/eu': quemSou });
    montar();
    await versao(1);
    await waitFor(() => expect(quemSou).toHaveBeenCalled());
    // Deixa a resposta de /api/eu ser lida antes de afirmar que o botão não existe.
    await new Promise((resolver) => setTimeout(resolver, 0));
    expect(screen.queryByRole('button', { name: voltarPara })).not.toBeInTheDocument();
  });

  it('showsServerProblemsWhenRestoreWouldBreakTheSkill', async () => {
    const problema = {
      codigo: 'FERRAMENTA_FORA_DA_BASE',
      nivel: 'erro',
      mensagem: 'A ferramenta saiu da base.',
    };
    responderCom({
      ...rotasBase(gestor),
      [`POST ${BASE}/voltar`]: () =>
        json({ codigo: 'FLUXO_INVALIDO', mensagem: 'fluxo inválido', problemas: [problema] }, 422),
    });
    const { user, onVoltou } = montar();
    const antiga = await versao(1);
    await user.click(await antiga.findByRole('button', { name: voltarPara }));
    await user.click(screen.getByRole('button', { name: 'Voltar' }));
    expect(
      await screen.findByText('Não dá para voltar: A ferramenta saiu da base.'),
    ).toBeInTheDocument();
    expect(onVoltou).not.toHaveBeenCalled();
  });
});
