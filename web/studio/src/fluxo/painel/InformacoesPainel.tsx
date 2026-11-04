import { Box, MenuItem, TextField, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import { type Informacao, InformacaoSchema } from '@/fluxo/modelo/tipos';
import { NovaInformacao } from './NovaInformacao';

type Props = {
  informacoes: readonly Informacao[];
  editavel: boolean;
  onMudar: (informacoes: Informacao[]) => void;
};

// '' e 'text' são o mesmo texto curto para o servidor; a tela mostra só 'text'.
const TIPOS_DE_INFORMACAO: Record<string, string> = {
  text: 'Texto curto',
  textarea: 'Texto longo',
  number: 'Número',
  date: 'Data',
  phone: 'Telefone',
  select: 'Escolha',
  checkbox: 'Sim/não',
  list: 'Lista',
};

const tipoMostrado = (tipo: string): string => tipo || 'text';

const TIPO_ESCOLHA = 'select';

/** Só Escolha tem opções: trocar o tipo não pode deixar opções órfãs indo para o servidor. */
const semOpcoesForaDeEscolha = (tipo: string): Partial<Informacao> =>
  tipo === TIPO_ESCOLHA ? { tipo } : { tipo, opcoes: [] };

function CabecalhoDasInformacoes(): ReactNode {
  return (
    <Box>
      <Typography variant="subtitle1" component="h2" sx={{ fontWeight: 700 }}>
        Informações que a skill pede
      </Typography>
      <Typography variant="caption" color="text.secondary">
        O que a skill pode perguntar ao morador.
      </Typography>
    </Box>
  );
}

export function InformacoesPainel({ informacoes, editavel, onMudar }: Props): ReactNode {
  const mudar = (nome: string, parcial: Partial<Informacao>) =>
    onMudar(
      informacoes.map((informacao) =>
        informacao.nome === nome ? { ...informacao, ...parcial } : informacao,
      ),
    );

  return (
    <Box
      component="section"
      aria-label="Informações que a skill pede"
      sx={{ display: 'flex', flexDirection: 'column', gap: 2, p: 2 }}
    >
      <CabecalhoDasInformacoes />
      {informacoes.map((informacao) => (
        <CartaoDeInformacao
          key={informacao.nome}
          info={informacao}
          editavel={editavel}
          onMudar={(parcial) => mudar(informacao.nome, parcial)}
        />
      ))}
      {editavel && (
        <NovaInformacao
          existentes={informacoes.map((informacao) => informacao.nome)}
          onAdicionar={(nome) => onMudar([...informacoes, InformacaoSchema.parse({ nome })])}
        />
      )}
    </Box>
  );
}

type CartaoProps = {
  info: Informacao;
  editavel: boolean;
  onMudar: (parcial: Partial<Informacao>) => void;
};

type TipoProps = {
  tipo: string;
  editavel: boolean;
  onMudar: (parcial: Partial<Informacao>) => void;
};

function TipoDaInformacao({ tipo, editavel, onMudar }: TipoProps): ReactNode {
  return (
    <TextField
      select
      size="small"
      label="Tipo"
      value={tipo}
      disabled={!editavel}
      onChange={(evento) => onMudar(semOpcoesForaDeEscolha(evento.target.value))}
    >
      {Object.entries(TIPOS_DE_INFORMACAO).map(([valor, rotulo]) => (
        <MenuItem key={valor} value={valor}>
          {rotulo}
        </MenuItem>
      ))}
      {!Object.hasOwn(TIPOS_DE_INFORMACAO, tipo) && <MenuItem value={tipo}>{tipo}</MenuItem>}
    </TextField>
  );
}

type OpcoesProps = Pick<Informacao, 'opcoes'> & {
  editavel: boolean;
  onMudar: (parcial: Partial<Informacao>) => void;
};

function OpcoesDaInformacao({ opcoes, editavel, onMudar }: OpcoesProps): ReactNode {
  return (
    <ListaEditavel
      titulo="Opções"
      adicionar="Adicionar opção"
      linhas={opcoes}
      desabilitado={!editavel}
      criarLinha={() => ({ valor: '', rotulo: '' })}
      onMudar={(novas) => onMudar({ opcoes: novas })}
      renderLinha={(opcao, mudar) => (
        <>
          <TextField
            size="small"
            label="Valor"
            value={opcao.valor}
            disabled={!editavel}
            onChange={(evento) => mudar({ valor: evento.target.value })}
          />
          <TextField
            size="small"
            label="Rótulo"
            value={opcao.rotulo}
            disabled={!editavel}
            onChange={(evento) => mudar({ rotulo: evento.target.value })}
          />
        </>
      )}
    />
  );
}

function CartaoDeInformacao({ info, editavel, onMudar }: CartaoProps): ReactNode {
  const tipo = tipoMostrado(info.tipo);
  return (
    <Box
      component="article"
      aria-label={info.titulo || (editavel ? info.nome : 'Informação sem nome')}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
        p: 1.5,
        border: 1,
        borderColor: 'divider',
        borderRadius: 1,
      }}
    >
      {editavel && (
        <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
          {info.nome}
        </Typography>
      )}
      <TextField
        size="small"
        label="Título"
        value={info.titulo}
        disabled={!editavel}
        onChange={(evento) => onMudar({ titulo: evento.target.value })}
      />
      <TextField
        size="small"
        label="Pergunta"
        placeholder="Ex.: Para que dia é a reserva?"
        value={info.pergunta}
        disabled={!editavel}
        multiline
        onChange={(evento) => onMudar({ pergunta: evento.target.value })}
      />
      <TipoDaInformacao tipo={tipo} editavel={editavel} onMudar={onMudar} />
      {tipo === TIPO_ESCOLHA && (
        <OpcoesDaInformacao opcoes={info.opcoes} editavel={editavel} onMudar={onMudar} />
      )}
    </Box>
  );
}
