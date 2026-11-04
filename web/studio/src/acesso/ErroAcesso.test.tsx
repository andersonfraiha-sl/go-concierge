import { screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { renderComProvedores } from '@/test/render';
import { ErroAcesso } from './ErroAcesso';

describe('ErroAcesso', () => {
  it('offersRetry', () => {
    renderComProvedores(<ErroAcesso aoTentar={() => {}} />);

    expect(screen.getByRole('alert')).toHaveTextContent('Não foi possível carregar seu acesso.');
    expect(screen.getByRole('button', { name: 'Tentar de novo' })).toBeEnabled();
  });
});
