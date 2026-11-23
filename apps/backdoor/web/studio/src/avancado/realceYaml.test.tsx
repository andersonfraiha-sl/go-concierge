import { render } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { realcarYaml } from './realceYaml';

describe('realcarYaml', () => {
  it('colorsTheKeyBeforeAColon', () => {
    const { container } = render(<pre>{realcarYaml('nome: concierge', '#000')}</pre>);
    expect(container.textContent).toBe('nome: concierge');
    expect(container.querySelector('span span')?.textContent).toBe('nome');
  });

  it('colorsAWholeCommentLine', () => {
    const { container } = render(<pre>{realcarYaml('# nota', '#111')}</pre>);
    const span = container.querySelector('span');
    expect(span?.textContent).toBe('# nota');
    expect(span).toHaveStyle({ color: '#111' });
  });

  it('leavesAPlainLineUntouched', () => {
    const { container } = render(<pre>{realcarYaml('  - unidades.listar', '#000')}</pre>);
    expect(container.textContent).toBe('  - unidades.listar');
    expect(container.querySelector('span span')).toBeNull();
  });
});
