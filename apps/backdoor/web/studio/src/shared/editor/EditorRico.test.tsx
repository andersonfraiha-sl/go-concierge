import { screen, waitFor } from '@testing-library/react';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { renderComProvedores } from '@/test/render';
import { EditorRico } from './EditorRico';

/** Controlado como no react-hook-form: o que sai pelo `onChange` volta como `valor`. */
function Controlado({
  inicial,
  editavel,
  aoMudar,
}: {
  inicial: string;
  editavel: boolean;
  aoMudar: (md: string) => void;
}) {
  const [valor, setValor] = useState(inicial);
  const mudar = (markdown: string) => {
    setValor(markdown);
    aoMudar(markdown);
  };
  return <EditorRico valor={valor} onChange={mudar} rotulo="Regra" editavel={editavel} />;
}

const montar = (valor: string, editavel = true) => {
  const onChange = vi.fn<(markdown: string) => void>();
  const tela = renderComProvedores(
    <Controlado inicial={valor} editavel={editavel} aoMudar={onChange} />,
  );
  return { ...tela, onChange };
};

describe('EditorRico', () => {
  it.each([
    { botao: 'Título', esperado: '## Seja gentil.\n' },
    { botao: 'Subtítulo', esperado: '### Seja gentil.\n' },
    { botao: 'Lista com marcadores', esperado: '- Seja gentil.\n' },
    { botao: 'Lista numerada', esperado: '1. Seja gentil.\n' },
  ])('turnsLineInto $botao', async ({ botao, esperado }) => {
    const { user, onChange } = montar('Seja gentil.\n');
    await user.click(screen.getByRole('button', { name: botao }));
    expect(onChange).toHaveBeenLastCalledWith(esperado);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: botao })).toHaveAttribute('aria-pressed', 'true'),
    );
  });

  it('underlinesTypedTextAsHtmlTag', async () => {
    const { user, onChange } = montar('Seja gentil.\n');
    await user.click(screen.getByRole('textbox', { name: 'Regra' }));
    await user.keyboard('{Control>}u{/Control}');
    await user.keyboard('Sempre');
    expect(onChange).toHaveBeenLastCalledWith('<u>Sempre</u>Seja gentil.\n');
  });

  it('keepsUntouchedBlocksByteForByte', async () => {
    const original =
      '# Políticas\n\n- Use `chamar_recurso`; A & B > C.\n  continua aqui.\n\nFim.\n';
    const { user, onChange } = montar(original);
    await user.click(screen.getByRole('button', { name: 'Título' }));
    expect(onChange).toHaveBeenLastCalledWith(original.replace('# Políticas', '## Políticas'));
  });

  it('doesNotEmitWithoutEdits', () => {
    const { onChange } = montar('# Título\n\nTexto com `código`.\n');
    expect(screen.getByRole('textbox', { name: 'Regra' })).toHaveTextContent('Texto com código.');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('readsOnlyWithoutToolbar', () => {
    montar('**Negrito** e texto.\n', false);
    const caixa = screen.getByRole('textbox', { name: 'Regra' });
    expect(caixa).toHaveAttribute('contenteditable', 'false');
    expect(caixa).toHaveAttribute('aria-readonly', 'true');
    expect(caixa.querySelector('strong')).toHaveTextContent('Negrito');
    expect(screen.queryByRole('toolbar')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Editar como texto' })).not.toBeInTheDocument();
  });

  it('fallsBackToPlainTextForTables', async () => {
    const tabela = '| a | b |\n|---|---|\n| 1 | 2 |\n';
    const { user, onChange } = montar(tabela);
    const campo = screen.getByRole('textbox', { name: 'Regra' });
    expect(campo).toHaveValue(tabela);
    expect(screen.queryByRole('toolbar')).not.toBeInTheDocument();
    expect(screen.getByText(/formatação que o editor não mostra/)).toBeInTheDocument();
    await user.type(campo, 'x');
    expect(onChange).toHaveBeenLastCalledWith(`${tabela}x`);
  });

  it('switchesToPlainTextAndBack', async () => {
    const { user } = montar('**Negrito**\n');
    await user.click(screen.getByRole('button', { name: 'Editar como texto' }));
    expect(screen.getByRole('textbox', { name: 'Regra' })).toHaveValue('**Negrito**\n');
    await user.click(screen.getByRole('button', { name: 'Voltar ao editor' }));
    await waitFor(() => expect(screen.getByRole('toolbar')).toBeInTheDocument());
  });
});
