import type { Page } from '@playwright/test';

// Mesmos quatro usuários de dev que o DevRoleSwitch oferece (ver src/acesso/DevRoleSwitch.tsx).
export const USUARIOS_DEV = [
  'admin-superlogica',
  'construtor-superlogica',
  'gestor-exemplo',
  'gestor-avancado',
] as const;

export type UsuarioDev = (typeof USUARIOS_DEV)[number];

/** Define o "Ver como" antes de qualquer navegação, igual ao localStorage que o app lê. */
export async function entrarComo(page: Page, usuario: UsuarioDev): Promise<void> {
  await page.addInitScript((valor) => localStorage.setItem('studio.usuarioDev', valor), usuario);
}
