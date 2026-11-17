import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import type { NoDaArvore } from './contratos';
import { IdentidadeAba } from './IdentidadeAba';

const noConcierge: NoDaArvore = {
  caminho: 'concierge',
  nome: 'concierge',
  tipo: 'agente',
  descricao: '',
  caso: false,
  erros: 0,
  avisos: 0,
  subagentes: [],
};

const noSkill: NoDaArvore = {
  ...noConcierge,
  caminho: 'skills/agendar-mudanca',
  nome: 'agendar-mudanca',
  caso: true,
};

const agenteDetalhe = (tipo: string, tipoDeclarado: boolean, modelo = '') => ({
  agente: {
    caminho: noConcierge.caminho,
    nome: noConcierge.nome,
    descricao: '',
    persona: '',
    tipo,
    tipo_declarado: tipoDeclarado,
    modelo,
    arquivos: {},
    herdados: {},
    problemas: [],
  },
  recursos: [],
});

const textosDaSkill = {
  caminho: noSkill.caminho,
  textos: {
    titulo: 'Agendamento de mudança',
    quandoUsar: 'Quando o morador pede para agendar mudança',
    frasesDeExemplo: [],
    palavrasChave: [],
    temFluxo: true,
    resumo: '',
    visibilidade: '',
  },
  versao: 'v1',
};

function capturarCorpos(chave: string, resposta: unknown) {
  const corpos: unknown[] = [];
  const rota = async (req: Request) => {
    corpos.push(await req.json());
    return json(resposta);
  };
  return { corpos, rotas: { [chave]: rota } };
}

describe('IdentidadeAba', () => {
  it('savesTypeAndModel', async () => {
    const { corpos, rotas } = capturarCorpos('PUT /api/agente/config', { salvo: 'concierge' });
    responderCom({ 'GET /api/agente': () => json(agenteDetalhe('', false)), ...rotas });

    const { user } = renderComProvedores(<IdentidadeAba no={noConcierge} editar={true} />);

    await user.click(await screen.findByRole('combobox', { name: 'Tipo' }));
    await user.click(within(screen.getByRole('listbox')).getByText('Orquestrador'));
    await user.type(screen.getByRole('textbox', { name: 'Modelo' }), 'gemini-2.5-flash');
    await user.click(screen.getByRole('button', { name: 'Salvar' }));

    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toEqual({
      agente: 'concierge',
      campos: { tipo: 'orquestrador', modelo: 'gemini-2.5-flash' },
    });
  });

  it('savesVisibilityForSkill', async () => {
    const config = capturarCorpos('PUT /api/agente/config', { salvo: noSkill.caminho });
    const textos = capturarCorpos('PUT /api/skills/agendar-mudanca/textos', { versao: 'v2' });
    responderCom({
      'GET /api/agente': () => json(agenteDetalhe('operador', true, 'gemini-2.5-flash')),
      'GET /api/skills/agendar-mudanca': () => json(textosDaSkill),
      ...config.rotas,
      ...textos.rotas,
    });

    const { user } = renderComProvedores(<IdentidadeAba no={noSkill} editar={true} />);

    await user.click(await screen.findByRole('radio', { name: 'Qualquer contato' }));
    await user.click(screen.getByRole('button', { name: 'Salvar' }));

    await waitFor(() => expect(textos.corpos).toHaveLength(1));
    expect(textos.corpos[0]).toEqual({
      versao: 'v1',
      textos: { ...textosDaSkill.textos, visibilidade: 'public' },
    });
  });

  it('hidesVisibilityWithoutFlow', async () => {
    responderCom({ 'GET /api/agente': () => json(agenteDetalhe('', false)) });

    renderComProvedores(<IdentidadeAba no={noConcierge} editar={true} />);

    await screen.findByRole('button', { name: 'Salvar' });
    expect(screen.queryByText('Visibilidade')).not.toBeInTheDocument();
    expect(screen.queryByRole('radio')).not.toBeInTheDocument();
  });

  it('readOnlyWithoutEditPermission', async () => {
    responderCom({
      'GET /api/agente': () => json(agenteDetalhe('operador', true, 'gemini-2.5-flash')),
      'GET /api/skills/agendar-mudanca': () => json(textosDaSkill),
    });

    renderComProvedores(<IdentidadeAba no={noSkill} editar={false} />);

    expect(await screen.findByRole('combobox', { name: 'Tipo' })).toHaveAttribute(
      'aria-disabled',
      'true',
    );
    expect(screen.getByRole('textbox', { name: 'Modelo' })).toBeDisabled();
    expect(screen.getByRole('radio', { name: 'Qualquer contato' })).toBeDisabled();
    expect(screen.queryByRole('button', { name: 'Salvar' })).not.toBeInTheDocument();
  });
});
