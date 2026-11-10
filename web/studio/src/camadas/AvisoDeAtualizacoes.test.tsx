import { screen, waitFor } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { json, responderCom } from '@/test/fetchFalso';
import { renderComProvedores } from '@/test/render';
import { AvisoDeAtualizacoes } from './AvisoDeAtualizacoes';
import type { Camada } from './contratos';

const camada: Camada = { skill: 'agendar-mudanca', tipo: 'administradora', id: 'adm-1' };

const atualizacao = {
  id: 'at-1',
  camada,
  estado: 'pendente',
  para: { numero: 8, resumo: 'Textos novos' },
  prazoEm: null,
  trechos: 4,
  conflitos: 1,
};

const aviso = {
  id: 'av-1',
  camada,
  motivo: 'a ferramenta de agenda mudou de nome.',
  trechos: ['texto:resumo', 'etapa:e-1'],
  encerraEm: null,
  criadoEm: '2026-11-04T10:00:00Z',
};

const montar = () => renderComProvedores(<AvisoDeAtualizacoes camada={camada} />);

describe('AvisoDeAtualizacoes', () => {
  it('showsUpdateBannerWithReviewLink', async () => {
    responderCom({
      'GET /api/atualizacoes': () => json({ atualizacoes: [atualizacao], avisos: [] }),
    });
    montar();
    expect(await screen.findByText('Essa skill tem atualizações')).toBeInTheDocument();
    expect(
      screen.getByText('Nova versão 8: Textos novos. 4 trechos para revisar, 1 em conflito.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Revisar' })).toHaveAttribute(
      'href',
      '/atualizacoes/at-1',
    );
  });

  it('hidesUpdatesOfOtherLayers', async () => {
    const deOutra = { ...atualizacao, id: 'at-2', camada: { ...camada, id: 'adm-2' } };
    responderCom({
      'GET /api/atualizacoes': () => json({ atualizacoes: [deOutra, atualizacao], avisos: [] }),
    });
    montar();
    expect(await screen.findByRole('link', { name: 'Revisar' })).toHaveAttribute(
      'href',
      '/atualizacoes/at-1',
    );
    expect(screen.getAllByRole('link', { name: 'Revisar' })).toHaveLength(1);
  });

  it('showsMandatoryNoticeWithReason', async () => {
    responderCom({ 'GET /api/atualizacoes': () => json({ atualizacoes: [], avisos: [aviso] }) });
    montar();
    expect(
      await screen.findByText(
        /A Superlógica trocou o resumo e uma etapa do passo a passo: a ferramenta de agenda mudou de nome\./,
      ),
    ).toBeInTheDocument();
  });

  it('showsEndDateWhenSkillIsRetired', async () => {
    const encerrada = { ...aviso, encerraEm: '2026-12-01' };
    responderCom({
      'GET /api/atualizacoes': () => json({ atualizacoes: [], avisos: [encerrada] }),
    });
    montar();
    expect(
      await screen.findByText(/Esta skill para de funcionar em 01\/12\/2026\./),
    ).toBeInTheDocument();
  });

  it('dismissesNoticeOnAcknowledge', async () => {
    let avisos = [aviso];
    responderCom({
      'GET /api/atualizacoes': () => json({ atualizacoes: [], avisos }),
      'POST /api/avisos/av-1/ciente': () => {
        avisos = [];
        return new Response(null, { status: 204 });
      },
    });
    const { user } = montar();
    await user.click(await screen.findByRole('button', { name: 'Entendi' }));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Entendi' })).not.toBeInTheDocument(),
    );
  });
});
