import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ApiError } from '@/shared/api/http';
import { renderComProvedores } from '@/test/render';
import { ErroAcesso } from './ErroAcesso';

describe('ErroAcesso', () => {
  it('showsUnauthenticatedScreen', () => {
    const erro = new ApiError(401, 'NAO_AUTENTICADO', 'usuário não autenticado', null);
    renderComProvedores(<ErroAcesso erro={erro} aoTentar={() => {}} />);

    expect(screen.getByRole('alert')).toHaveTextContent('Não reconhecemos seu usuário.');
    expect(screen.getByText('Entre de novo para continuar.')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Tentar de novo' })).not.toBeInTheDocument();
  });

  it('keepsRetryForOtherErrors', () => {
    const erro = new ApiError(500, 'ERRO_INTERNO', 'falhou', null);
    renderComProvedores(<ErroAcesso erro={erro} aoTentar={() => {}} />);

    expect(screen.getByRole('alert')).toHaveTextContent('Não foi possível carregar seu acesso.');
    expect(screen.getByRole('button', { name: 'Tentar de novo' })).toBeEnabled();
  });
});
