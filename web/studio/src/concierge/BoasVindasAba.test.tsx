import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import type { Eu } from '@/acesso/eu';
import { caminhoDeRegra } from '@/catalogo/contratos';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { BoasVindasAba } from './BoasVindasAba';

const ASSISTENTE = 'concierge';
const CAMINHO_BOAS_VINDAS = caminhoDeRegra('boas-vindas.md');

const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['skills.ler', 'skills.editar_textos'],
  administradoraId: null,
  dev: false,
};

const textosLidos = (visibilidade = 'private') => ({
  caminho: ASSISTENTE,
  textos: {
    titulo: 'Concierge',
    quandoUsar: '',
    frasesDeExemplo: [],
    palavrasChave: [],
    temFluxo: false,
    resumo: '',
    visibilidade,
  },
  versao: 'v1',
});

const montar = () => renderComProvedores(<BoasVindasAba assistente={ASSISTENTE} />);

const rotasBase = (visibilidade = 'private') => ({
  'GET /api/eu': () => json(construtor),
  'GET /api/skills/concierge': () => json(textosLidos(visibilidade)),
  'GET /api/arquivo': () => json({ caminho: CAMINHO_BOAS_VINDAS, conteudo: 'Oi, morador!' }),
});

describe('BoasVindasAba', () => {
  it('savesWelcomeTextOneSecondAfterTyping', async () => {
    const corpos: unknown[] = [];
    responderCom({
      ...rotasBase(),
      'PUT /api/arquivo': async (req: Request) => {
        corpos.push(await req.json());
        return json({ salvo: CAMINHO_BOAS_VINDAS });
      },
    });
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Mensagem de boas-vindas' });
    await waitFor(() => expect(campo).toBeEnabled());
    await user.type(campo, ' Tudo bem?');
    expect(corpos).toEqual([]);
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos).toEqual([
      { agente: ASSISTENTE, caminho: CAMINHO_BOAS_VINDAS, conteudo: 'Oi, morador! Tudo bem?' },
    ]);
  });

  it('createsWelcomeFileWhenMissing', async () => {
    const corpos: unknown[] = [];
    responderCom({
      ...rotasBase(),
      'GET /api/arquivo': () => json({ codigo: 'NAO_ENCONTRADO', mensagem: 'sem arquivo' }, 404),
      'PUT /api/arquivo': async (req: Request) => {
        corpos.push(await req.json());
        return json({ salvo: CAMINHO_BOAS_VINDAS });
      },
    });
    const { user } = montar();
    const campo = await screen.findByRole('textbox', { name: 'Mensagem de boas-vindas' });
    expect(campo).toHaveValue('');
    await user.type(campo, 'Oi!');
    await waitFor(() => expect(corpos).toHaveLength(1), { timeout: 3000 });
    expect(corpos).toEqual([{ agente: ASSISTENTE, caminho: CAMINHO_BOAS_VINDAS, conteudo: 'Oi!' }]);
  });

  it('savesWhoCanTalkAsVisibility', async () => {
    const corpos: unknown[] = [];
    responderCom({
      ...rotasBase(),
      'PUT /api/skills/concierge/textos': async (req: Request) => {
        corpos.push(await req.json());
        return json({ versao: 'v2' });
      },
    });
    const { user } = montar();
    const qualquerContato = await screen.findByRole('button', { name: 'Qualquer contato' });
    await user.click(qualquerContato);
    await waitFor(() => expect(corpos).toHaveLength(1));
    expect(corpos[0]).toEqual({
      versao: 'v1',
      textos: { ...textosLidos().textos, visibilidade: 'public' },
    });
  });

  it('gestorReadsOnly', async () => {
    const gestor: Eu = { ...construtor, papel: 'gestor', administradoraId: 'adm-1' };
    responderCom({ ...rotasBase(), 'GET /api/eu': () => json(gestor) });
    montar();
    const campo = await screen.findByRole('textbox', { name: 'Mensagem de boas-vindas' });
    expect(campo).toBeDisabled();
    expect(await screen.findByRole('button', { name: 'Qualquer contato' })).toBeDisabled();
  });
});
