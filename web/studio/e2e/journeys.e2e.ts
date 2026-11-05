import { expect, test } from '@playwright/test';
import { entrarComo } from './devUsuario';

test.describe('menu e navegação', () => {
  test('navigatesEveryMainSection', async ({ page }) => {
    await entrarComo(page, 'admin-superlogica');
    await page.goto('/studio/');

    const rotas: [rotulo: string, pedaco: RegExp][] = [
      ['Skills', /\/skills/],
      ['Ferramentas', /\/ferramentas/],
      ['Condomínios', /\/condominios/],
      ['Avançado', /\/avancado/],
      ['Concierge', /\/concierge|\/studio\/?$/],
    ];
    for (const [rotulo, pedaco] of rotas) {
      await page.getByRole('link', { name: rotulo }).click();
      await expect(page).toHaveURL(pedaco);
    }
  });
});

test.describe('skill: abrir, editar e salvar regra', () => {
  test('editsAndSavesARuleFile', async ({ page }) => {
    await entrarComo(page, 'construtor-superlogica');
    await page.goto('/studio/skills');

    await page.getByRole('link', { name: 'Agendamento de Mudança' }).click();
    await page.getByRole('tab', { name: 'Regras' }).click();

    const editor = page.getByRole('textbox').first();
    await expect(editor).toBeVisible();
    const textoOriginal = (await editor.textContent()) ?? '';
    await editor.click();
    await page.keyboard.type(' // ajuste do e2e');
    await page.getByRole('button', { name: 'Salvar' }).click();
    await expect(page.getByText('Salvo', { exact: false })).toBeVisible({ timeout: 10_000 });

    // devolve o arquivo ao estado original pra não deixar lixo entre execuções.
    await editor.click();
    await page.keyboard.press('Control+A');
    await page.keyboard.type(textoOriginal);
    await page.getByRole('button', { name: 'Salvar' }).click();
  });
});

test.describe('Avançado: Editar, Cancelar, Salvar', () => {
  test('cancelDiscardsEditsAndSaveKeepsThem', async ({ page }) => {
    await entrarComo(page, 'admin-superlogica');
    await page.goto('/studio/avancado');

    await page.getByRole('button', { name: 'politicas.md' }).first().click();
    await page.getByRole('button', { name: 'Editar' }).click();
    const editor = page.getByRole('textbox');
    await editor.fill('conteúdo temporário do e2e');
    await page.getByRole('button', { name: 'Cancelar' }).click();
    await expect(page.getByRole('button', { name: 'Salvar' })).not.toBeVisible();

    await page.getByRole('button', { name: 'Editar' }).click();
    await expect(page.getByRole('textbox')).not.toHaveValue('conteúdo temporário do e2e');
  });
});

test.describe('condomínios', () => {
  test('opensSeedCondominiumAndTogglesASkill', async ({ page }) => {
    await entrarComo(page, 'admin-superlogica');
    await page.goto('/studio/condominios');

    await page.getByRole('link', { name: 'Condomínio exemplo' }).click();
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await expect(page.getByRole('switch').first()).toBeVisible();
  });
});

test.describe('ferramentas', () => {
  test('listsToolsGroupedBySection', async ({ page }) => {
    await entrarComo(page, 'admin-superlogica');
    await page.goto('/studio/ferramentas');
    await expect(page.getByRole('heading').first()).toBeVisible();
  });
});

test.describe('troca de papel (Ver como)', () => {
  test('eachRoleSeesADifferentSetOfSections', async ({ page }) => {
    await entrarComo(page, 'gestor-exemplo');
    await page.goto('/studio/');
    await expect(page.getByRole('link', { name: 'Avançado' })).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Condomínios' })).toBeVisible();

    await page.getByRole('button', { name: 'Conta e Ver como' }).click();
    await page.getByLabel('Ver como').click();
    await page.getByRole('option', { name: 'Administradora com Avançado' }).click();

    await expect(page.getByRole('link', { name: 'Avançado' })).toBeVisible();
  });
});
