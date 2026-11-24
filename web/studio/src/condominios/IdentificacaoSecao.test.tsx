import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { responderCom } from '@/test/fetchFalso';
import { construtor, gestor, rotasBase } from './estadoFalso';
import { abrirCondominio } from './paginaDeTeste';

describe('identidade externa no conteúdo', () => {
  it.each([construtor, gestor])('naoPermiteEditarIdentidadeOuCanalPara $papel', async (eu) => {
    responderCom(rotasBase(eu));
    abrirCondominio();

    await screen.findByRole('heading', { name: 'Conteúdo por skill' });
    expect(screen.queryByText('Identificação')).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Nome' })).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Empresa no Gruvi' })).not.toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Canal do Atende' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Remover condomínio' })).not.toBeInTheDocument();
  });
});
