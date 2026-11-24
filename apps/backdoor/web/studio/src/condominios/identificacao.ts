import type { Condominio } from './contratos';

export const CAMPOS_DE_IDENTIFICACAO = ['nome', 'account', 'company_id', 'tenant'] as const;
export type CampoDeIdentificacao = (typeof CAMPOS_DE_IDENTIFICACAO)[number];

const CAMPOS_COM_FORMATO = ['account', 'company_id', 'tenant'] as const;
type CampoDeFormato = (typeof CAMPOS_COM_FORMATO)[number];

const FORMATOS: Record<CampoDeFormato, { regex: RegExp; mensagem: string }> = {
  account: {
    regex: /^[a-z0-9][a-z0-9-]*$/,
    mensagem: 'Use letras minúsculas, números e hífen, como cond-001.',
  },
  company_id: {
    regex: /^\d+$/,
    mensagem: 'Use só números, como 5982.',
  },
  tenant: {
    regex: /^[0-9a-f]{24}$/,
    mensagem: 'São 24 caracteres, de 0 a 9 e de a a f.',
  },
};

/** Nome é obrigatório; account/company_id/tenant são opcionais, mas quando preenchidos seguem o formato. */
export function validarIdentificacao(
  condominio: Condominio,
): Partial<Record<CampoDeIdentificacao, string>> {
  const erros: Partial<Record<CampoDeIdentificacao, string>> = {};
  if (condominio.nome.trim() === '') erros.nome = 'Informe o nome.';

  for (const campo of CAMPOS_COM_FORMATO) {
    const valor = condominio[campo];
    const { regex, mensagem } = FORMATOS[campo];
    if (valor !== '' && !regex.test(valor)) erros[campo] = mensagem;
  }

  return erros;
}

/**
 * O que ir para o servidor: cada campo inválido volta para o último valor salvo, os demais
 * seguem a edição atual. Um campo com formato errado nunca trava os outros nem apaga a digitação
 * na tela (quem chama continua mostrando `atual`, só o envio usa este resultado).
 */
export function payloadSeguro(atual: Condominio, ultimoSalvo: Condominio): Condominio {
  const erros = validarIdentificacao(atual);
  const seguro = { ...atual };
  for (const campo of CAMPOS_DE_IDENTIFICACAO) {
    if (erros[campo] !== undefined) seguro[campo] = ultimoSalvo[campo];
  }
  return seguro;
}
