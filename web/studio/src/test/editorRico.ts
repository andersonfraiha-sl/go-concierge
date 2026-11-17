import type { UserEvent } from '@testing-library/user-event';

/** Digita no fim do editor rico: o jsdom não leva o cursor ao fim sozinho como o navegador. */
export async function digitarNoFim(
  user: UserEvent,
  caixa: HTMLElement,
  texto: string,
): Promise<void> {
  await user.click(caixa);
  const fim = document.createRange();
  fim.selectNodeContents(caixa.lastElementChild ?? caixa);
  fim.collapse(false);
  document.getSelection()?.removeAllRanges();
  document.getSelection()?.addRange(fim);
  document.dispatchEvent(new Event('selectionchange'));
  await user.keyboard(texto);
}
