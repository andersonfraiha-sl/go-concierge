import { useQuery, useQueryClient } from '@tanstack/react-query';
import { z } from 'zod';
import { useEu } from '@/acesso/eu';
import { type Ferramenta, FerramentaSchema } from '@/catalogo/contratos';
import { apiEnviar, apiGet } from '@/shared/api/http';
import { listaDoGo } from '@/shared/api/listaDoGo';

/** Chave do mapeamento comum a todos os condomínios. */
export const PADRAO = '*';

/** Id da administradora do usuário (null = vê tudo): vai como ?administradora= em toda chamada. */
export type Escopo = string | null;

const AdministradoraSchema = z.object({ id: z.string(), nome: z.string() });
export type Administradora = z.infer<typeof AdministradoraSchema>;

const CondominioSchema = z.object({
  id: z.string(),
  nome: z.string(),
  administradora_id: z.string().default(''),
  account: z.string().default(''),
  company_id: z.string().default(''),
  tenant: z.string().default(''),
});
export type Condominio = z.infer<typeof CondominioSchema>;

const HabilitacaoSchema = z.object({ habilitado: z.boolean(), papeis: listaDoGo(z.string()) });
export type Habilitacao = z.infer<typeof HabilitacaoSchema>;

const ValorSchema = z.json();
export type Valor = z.infer<typeof ValorSchema>;
export type Ajustes = Record<string, Record<string, Valor>>;

/** O Go serializa mapa nil como null: a borda entrega sempre mapa. */
const mapaDoGo = <S extends z.ZodType>(item: S) =>
  z
    .record(z.string(), item)
    .nullish()
    .transform((mapa) => mapa ?? {});

const OrientacaoSchema = z.object({ geral: z.string(), por_caso: mapaDoGo(z.string()) });
export type Orientacao = z.infer<typeof OrientacaoSchema>;

const MapeamentoSchema = z.object({
  administradoras: listaDoGo(AdministradoraSchema),
  condominios: listaDoGo(CondominioSchema),
  habilitacoes: mapaDoGo(mapaDoGo(HabilitacaoSchema)),
  payloads: mapaDoGo(mapaDoGo(mapaDoGo(ValorSchema))),
  orientacoes: mapaDoGo(OrientacaoSchema),
  // Hora da última gravação; a ficha usa como versão para ignorar leitura mais velha.
  atualizado_em: z.string().optional(),
});
export type Mapeamento = z.infer<typeof MapeamentoSchema>;

const CasoSchema = z.object({
  nome: z.string(),
  titulo: z.string(),
  descricao: z.string(),
  escrita: z.boolean(),
  recursos: listaDoGo(z.string()),
  id: z.string().default(''),
  resumo: z.string().default(''),
  icone: z.string().default(''),
});
export type Caso = z.infer<typeof CasoSchema>;

export const EstadoSchema = z.object({
  mapeamento: MapeamentoSchema,
  administradoras: listaDoGo(AdministradoraSchema),
  casos: listaDoGo(CasoSchema),
  recursos: listaDoGo(FerramentaSchema),
  papeis: listaDoGo(z.string()),
});
export type Estado = z.infer<typeof EstadoSchema>;

const RespostaMapeamentoSchema = z.object({
  mapeamento: MapeamentoSchema,
  administradoras: listaDoGo(AdministradoraSchema),
});
export type RespostaMapeamento = z.infer<typeof RespostaMapeamentoSchema>;

const SugestaoSchema = z.object({
  payload: mapaDoGo(ValorSchema),
  nota: z.string().optional(),
});
export type Sugestao = z.infer<typeof SugestaoSchema>;

function caminhoAdmin(rota: string, escopo: Escopo, extras: Record<string, string> = {}): string {
  const consulta = new URLSearchParams(extras);
  if (escopo !== null) consulta.set('administradora', escopo);
  const texto = consulta.toString();
  return texto === '' ? `/api/admin/${rota}` : `/api/admin/${rota}?${texto}`;
}

const gravar = (rota: string, escopo: Escopo, corpo: unknown): Promise<RespostaMapeamento> =>
  apiEnviar('PUT', caminhoAdmin(rota, escopo), corpo, RespostaMapeamentoSchema);

const apagar = (rota: string, escopo: Escopo, id: string): Promise<RespostaMapeamento> =>
  apiEnviar('DELETE', caminhoAdmin(rota, escopo, { id }), undefined, RespostaMapeamentoSchema);

export const salvarCondominio = (condominio: Condominio, escopo: Escopo) =>
  gravar('condominio', escopo, condominio);

export const removerCondominio = (id: string, escopo: Escopo) => apagar('condominio', escopo, id);

export const salvarHabilitacao = (
  corpo: {
    condominio: string;
    caso: string;
    habilitado: boolean;
    papeis: string[];
    limpar: boolean;
  },
  escopo: Escopo,
) => gravar('habilitacao', escopo, corpo);

export const salvarOrientacao = (
  corpo: { condominio: string; geral: string; por_caso: Record<string, string> },
  escopo: Escopo,
) => gravar('orientacao', escopo, corpo);

export const salvarAjustes = (corpo: { condominio: string; ajustes: Ajustes }, escopo: Escopo) =>
  gravar('ajustes', escopo, corpo);

export const salvarPayload = (
  corpo: { condominio: string; recurso: string; payload: Record<string, Valor>; apagar: boolean },
  escopo: Escopo,
) => gravar('payload', escopo, corpo);

export const salvarAdministradora = (administradora: Administradora, escopo: Escopo) =>
  gravar('administradora', escopo, administradora);

export const removerAdministradora = (id: string, escopo: Escopo) =>
  apagar('administradora', escopo, id);

export function sugerirAjustes(
  corpo: { condominio: string; recurso: string; texto: string; atual: Record<string, Valor> },
  escopo: Escopo,
): Promise<Sugestao> {
  return apiEnviar('POST', caminhoAdmin('sugerir-payload', escopo), corpo, SugestaoSchema);
}

export function useEscopo(): Escopo | undefined {
  const { data: eu } = useEu();
  return eu?.administradoraId;
}

export const chaveDoEstado = (escopo: Escopo | undefined) =>
  ['estado-condominios', escopo] as const;

export function buscarEstado(escopo: Escopo): Promise<Estado> {
  return apiGet(caminhoAdmin('estado', escopo), EstadoSchema);
}

export function useEstado() {
  const escopo = useEscopo();
  return useQuery({
    queryKey: chaveDoEstado(escopo),
    queryFn: () => buscarEstado(escopo ?? null),
    enabled: escopo !== undefined,
  });
}

/** Aplica no cache a resposta de uma gravação, sem refazer o GET (o formulário aberto não reinicia). */
export function useAplicarMapeamento(): (resposta: RespostaMapeamento) => void {
  const cliente = useQueryClient();
  return (resposta) =>
    cliente.setQueriesData<Estado>(
      { queryKey: ['estado-condominios'] },
      (estado) =>
        estado && {
          ...estado,
          mapeamento: resposta.mapeamento,
          administradoras: resposta.administradoras,
        },
    );
}

/** Decisão do condomínio; sem ela vale o padrão; sem o padrão, a skill está ligada para todos os papéis. */
export function habilitacaoDe(
  mapeamento: Mapeamento,
  condominio: string,
  caso: string,
): { habilitacao: Habilitacao; propria: boolean } {
  const propria = mapeamento.habilitacoes[condominio]?.[caso];
  if (propria) return { habilitacao: propria, propria: true };
  const doPadrao = condominio === PADRAO ? undefined : mapeamento.habilitacoes[PADRAO]?.[caso];
  return { habilitacao: doPadrao ?? { habilitado: true, papeis: [] }, propria: false };
}

/** account, company_id ou tenant vazio (depois de trim) trava a integração deste condomínio. */
export function faltamIds(c: Condominio): boolean {
  return [c.account, c.company_id, c.tenant].some((valor) => valor.trim() === '');
}

/** Ferramentas com campo de ajuste que esta skill usa. */
export function ferramentasDaSkill(estado: Estado, caso: Caso): Ferramenta[] {
  return estado.recursos.filter(
    (recurso) => recurso.definicoes.length > 0 && caso.recursos.includes(recurso.name),
  );
}

/** Ferramentas com campo de ajuste que não pertencem a nenhuma skill (ajuste geral do condomínio). */
export function ferramentasGerais(estado: Estado): Ferramenta[] {
  const usadasPorAlgumaSkill = new Set(estado.casos.flatMap((caso) => caso.recursos));
  return estado.recursos.filter(
    (recurso) => recurso.definicoes.length > 0 && !usadasPorAlgumaSkill.has(recurso.name),
  );
}
