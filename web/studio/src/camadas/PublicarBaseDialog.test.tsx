import { screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { construtor, estadoFalso } from '@/condominios/estadoFalso';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { PublicarBaseDialog } from './PublicarBaseDialog';

const BASE = '/api/skills/agendar-mudanca/publicar';

const trecho = (id: string, rotulo: string, antes: string | null, depois: string | null) => ({
  trecho: id,
  tipo: 'texto',
  rotulo,
  situacao: 'so_base',
  obrigatorio: false,
  antes,
  depois,
  minha: null,
});

const previa = {
  numero: 3,
  diferencas: [
    trecho('texto:titulo', 'Título', 'Agendar mudança', 'Agendar mudança de unidade'),
    trecho('texto:resumo', 'Resumo', 'Agenda o dia.', 'Agenda o dia no prédio.'),
  ],
};

type Rota = (req: Request) => Response | Promise<Response>;

function montar(extras: Record<string, Rota> = {}, resposta = previa) {
  const onFechar = vi.fn();
  responderCom({
    [`GET ${BASE}`]: () => json(resposta),
    'GET /api/eu': () => json(construtor),
    'GET /api/admin/estado': () => json(estadoFalso),
    'GET /api/atualizacoes': () => json({ atualizacoes: [], avisos: [] }),
    ...extras,
  });
  const tela = renderComProvedores(
    <PublicarBaseDialog skill="agendar-mudanca" aberto onFechar={onFechar} />,
  );
  return { ...tela, onFechar };
}

const botaoPublicar = () => screen.findByRole('button', { name: 'Publicar' });

describe('PublicarBaseDialog', () => {
  it('listsChangedSectionsFromPreview', async () => {
    montar();

    const titulo = await screen.findByRole('region', { name: 'Título' });
    expect(within(titulo).getByText('Agendar mudança de unidade')).toBeInTheDocument();
    expect(within(titulo).getByText('Agendar mudança')).toBeInTheDocument();
    expect(screen.getByRole('region', { name: 'Resumo' })).toBeInTheDocument();
    expect(screen.getByText(/desde a versão 3/)).toBeInTheDocument();
  });

  it('saysNothingChanged', async () => {
    montar({}, { numero: 3, diferencas: [] });

    expect(await screen.findByText('Nada mudou desde a versão 3.')).toBeInTheDocument();
    expect(await botaoPublicar()).toBeDisabled();
  });

  it('requiresReasonWhenMandatory', async () => {
    const { user } = montar();

    await user.click(await screen.findByRole('checkbox', { name: 'Obrigatória: Título' }));
    expect(await botaoPublicar()).toBeDisabled();

    await user.type(screen.getByRole('textbox', { name: 'Motivo' }), 'Falha de segurança.');
    expect(await botaoPublicar()).toBeEnabled();
  });

  it('requiresReasonWhenRetiring', async () => {
    const { user } = montar();

    await user.click(await screen.findByRole('button', { name: 'Encerrar esta skill' }));
    await user.type(screen.getByLabelText('Encerra em'), '2026-12-31');
    expect(await botaoPublicar()).toBeDisabled();

    await user.type(screen.getByRole('textbox', { name: 'Motivo' }), 'Substituída por outra.');
    expect(await botaoPublicar()).toBeEnabled();
  });

  it('publishesAndReportsLayers', async () => {
    const corpos: unknown[] = [];
    const { user, onFechar } = montar({
      [`POST ${BASE}`]: async (req) => {
        corpos.push(await req.json());
        return json({ numero: 4, camadas: 2 }, 201);
      },
    });

    await user.click(await screen.findByRole('checkbox', { name: 'Obrigatória: Resumo' }));
    await user.type(screen.getByRole('textbox', { name: 'Resumo da versão' }), 'Textos novos.');
    await user.type(screen.getByRole('textbox', { name: 'Motivo' }), 'Texto errado.');
    await user.click(await botaoPublicar());

    expect(await screen.findByText('Versão 4 publicada para 2 camadas.')).toBeInTheDocument();
    expect(corpos).toEqual([
      {
        resumo: 'Textos novos.',
        obrigatorios: ['texto:resumo'],
        motivo: 'Texto errado.',
        encerraEm: null,
      },
    ]);
    expect(onFechar).toHaveBeenCalled();
  });

  it('sendsRetirementDate', async () => {
    const corpos: unknown[] = [];
    const { user } = montar({
      [`POST ${BASE}`]: async (req) => {
        corpos.push(await req.json());
        return json({ numero: 4, camadas: 1 }, 201);
      },
    });

    await user.click(await screen.findByRole('button', { name: 'Encerrar esta skill' }));
    await user.type(screen.getByLabelText('Encerra em'), '2026-12-31');
    await user.type(screen.getByRole('textbox', { name: 'Motivo' }), 'Substituída.');
    await user.click(await botaoPublicar());

    expect(await screen.findByText('Versão 4 publicada para 1 camada.')).toBeInTheDocument();
    expect(corpos).toMatchObject([{ encerraEm: '2026-12-31', obrigatorios: [] }]);
  });

  it('showsBrokenLayersOnConflict', async () => {
    const { user, onFechar } = montar({
      [`POST ${BASE}`]: () =>
        json(
          {
            codigo: 'CAMADAS_QUEBRAM',
            mensagem: 'Algumas camadas quebrariam.',
            camadas: [
              {
                tipo: 'administradora',
                id: 'adm-exemplo',
                problemas: [
                  { codigo: 'etapa_sem_destino', nivel: 'erro', mensagem: 'Etapa sem destino.' },
                ],
              },
              {
                tipo: 'condominio',
                id: 'vila-verde',
                problemas: [{ codigo: 'x', nivel: 'erro', mensagem: 'Ferramenta removida.' }],
              },
            ],
          },
          409,
        ),
    });

    await user.click(await botaoPublicar());

    const alerta = await screen.findByRole('alert');
    expect(alerta).toHaveTextContent('Administradora Administradora Exemplo: Etapa sem destino.');
    expect(alerta).toHaveTextContent('Condomínio Vila Verde: Ferramenta removida.');
    expect(onFechar).not.toHaveBeenCalled();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
  });

  it('showsFlowProblemsOnRejection', async () => {
    const { user } = montar({
      [`POST ${BASE}`]: () =>
        json(
          {
            codigo: 'FLUXO_INVALIDO',
            mensagem: 'Fluxo inválido.',
            problemas: [{ codigo: 'sem_resposta', nivel: 'erro', mensagem: 'Falta a resposta.' }],
          },
          422,
        ),
    });

    await user.click(await botaoPublicar());

    expect(await screen.findByRole('alert')).toHaveTextContent('Falta a resposta.');
  });

  it('showsServerMessageOnOtherFailures', async () => {
    const { user } = montar({
      [`POST ${BASE}`]: () =>
        json({ codigo: 'MOTIVO_OBRIGATORIO', mensagem: 'Escreva o motivo.' }, 422),
    });

    await user.click(await botaoPublicar());

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('Escreva o motivo.'));
  });
});
