import type { Eu } from '@/acesso/eu';
import { json } from '@/test/fetchFalso';

// Fixtures montadas pelos structs de internal/admin e cmd/okf-admin (slices nil viram null).
export const construtor: Eu = {
  id: 'u1',
  nome: 'Ana',
  papel: 'construtor',
  permissoes: ['condominios.habilitar', 'condominios.editar', 'administradoras.editar'],
  administradoraId: null,
  dev: true,
};

export const gestor: Eu = {
  id: 'u2',
  nome: 'Bia',
  papel: 'gestor',
  permissoes: ['condominios.habilitar'],
  administradoraId: 'adm-exemplo',
  dev: false,
};

export const estadoFalso = {
  mapeamento: {
    administradoras: [{ id: 'adm-exemplo', nome: 'Administradora Exemplo' }],
    condominios: [
      {
        id: 'vila-verde',
        nome: 'Vila Verde',
        administradora_id: 'adm-exemplo',
        account: 'acc-1',
        company_id: '100',
        tenant: 'aaaaaaaaaaaaaaaaaaaaaaaa',
      },
    ],
    habilitacoes: {
      '*': { 'reserva-do-salao': { habilitado: false, papeis: null } },
      'vila-verde': {},
    },
    payloads: {},
    orientacoes: {},
    atualizado_em: '2026-11-03T10:00:00Z',
  },
  administradoras: [{ id: 'adm-exemplo', nome: 'Administradora Exemplo' }],
  casos: [
    {
      nome: 'agendar-mudanca',
      id: 'agendar_mudanca',
      titulo: 'Agendar mudança',
      descricao: 'Marca a mudança do morador.',
      escrita: true,
      tipo: 'caso',
      recursos: ['agenda-mudanca'],
      resumo: 'Agenda o dia da mudança.',
      icone: 'truck',
    },
    {
      nome: 'reserva-do-salao',
      id: 'reserva_salao',
      titulo: 'Reserva do salão',
      descricao: 'Reserva o salão de festas.',
      escrita: true,
      tipo: 'caso',
      recursos: ['reserva-salao'],
      resumo: 'Reserva o salão de festas.',
      icone: '',
    },
  ],
  recursos: [
    {
      name: 'agenda-mudanca',
      titulo: 'Agenda de mudança',
      kind: 'http',
      visibility: 'private',
      write: true,
      description: 'Agenda a mudança.',
      params: null,
      definicoes: [
        {
          name: 'antecedencia_dias',
          label: 'Antecedência mínima (dias)',
          type: 'number',
          default: 2,
        },
        { name: 'turno', label: 'Turno', type: 'select', options: ['manhã', 'tarde'] },
        { name: 'exige_protocolo', label: 'Exige protocolo', type: 'boolean' },
      ],
    },
    {
      name: 'reserva-salao',
      titulo: 'Reserva do salão',
      kind: 'http',
      visibility: 'private',
      write: true,
      description: 'Reserva o salão de festas.',
      params: null,
      definicoes: null,
    },
    {
      name: 'horario-portaria',
      titulo: 'Horário da portaria',
      kind: 'http',
      visibility: 'private',
      write: false,
      description: 'Horário de atendimento da portaria.',
      params: null,
      definicoes: [
        { name: 'horario', label: 'Horário da portaria', type: 'string', default: '7h às 22h' },
      ],
    },
  ],
  papeis: ['morador', 'proprietario', 'sindico', 'publico'],
  account: 'acc-1',
  gruvi: { company_id: 'gruvi-1' },
};

export const skillsFalsas = {
  assistente: 'concierge',
  skills: [
    {
      nome: 'agendar-mudanca',
      id: 'agendar_mudanca',
      titulo: 'Agendar mudança',
      descricao: 'Marca a mudança do morador.',
      escrita: true,
      tipo: 'caso',
      recursos: ['agenda-mudanca'],
      caminho: 'agentes/concierge/agentes/agendar-mudanca',
      resumo: 'Agenda o dia da mudança.',
      icone: 'truck',
    },
    {
      nome: 'reserva-do-salao',
      id: 'reserva_salao',
      titulo: 'Reserva do salão',
      descricao: 'Reserva o salão de festas.',
      escrita: true,
      tipo: 'caso',
      recursos: ['reserva-salao'],
      caminho: 'agentes/concierge/agentes/reserva-do-salao',
      resumo: 'Reserva o salão de festas.',
      icone: '',
    },
  ],
};

export const rotasBase = (eu: Eu) => ({
  'GET /api/eu': () => json(eu),
  'GET /api/admin/estado': () => json(estadoFalso),
  'GET /api/skills': () => json(skillsFalsas),
});

export const respostaMapeamento = {
  mapeamento: estadoFalso.mapeamento,
  administradoras: estadoFalso.administradoras,
};
