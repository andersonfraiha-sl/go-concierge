import AddIcon from '@mui/icons-material/Add';
import { Box, Button, MenuItem, TextField, Typography } from '@mui/material';
import { useState, type ReactNode } from 'react';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import { InformacaoSchema, type Informacao } from '@/fluxo/modelo/tipos';
import { paraIdentificador } from '@/skills/contratos';

type Props = { informacoes: readonly Informacao[]; editavel: boolean; onMudar: (informacoes: Informacao[]) => void };

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

const nomeDaInformacao = (texto: string): string => paraIdentificador(texto).replaceAll('-', '_');

export function InformacoesPainel({ informacoes, editavel, onMudar }: Props): ReactNode {
  const mudar = (nome: string, parcial: Partial<Informacao>) =>
    onMudar(informacoes.map((info) => (info.nome === nome ? { ...info, ...parcial } : info)));

  return (
    <Box
      component="section"
      aria-label="Informações que a skill pede"
      sx={{ display: 'flex', flexDirection: 'column', gap: 2, p: 2 }}
    >
      <Box>
        <Typography variant="subtitle1" component="h2" sx={{ fontWeight: 700 }}>
          Informações que a skill pede
        </Typography>
        <Typography variant="caption" color="text.secondary">
          O que a skill pode perguntar ao morador.
        </Typography>
      </Box>
      {informacoes.map((info) => (
        <CartaoDeInformacao key={info.nome} info={info} editavel={editavel} onMudar={(parcial) => mudar(info.nome, parcial)} />
      ))}
      {editavel && (
        <NovaInformacao
          existentes={informacoes.map((i) => i.nome)}
          onAdicionar={(nome) => onMudar([...informacoes, InformacaoSchema.parse({ nome })])}
        />
      )}
    </Box>
  );
}

type CartaoProps = { info: Informacao; editavel: boolean; onMudar: (parcial: Partial<Informacao>) => void };

function CartaoDeInformacao({ info, editavel, onMudar }: CartaoProps): ReactNode {
  const tipo = tipoMostrado(info.tipo);
  return (
    <Box
      component="article"
      aria-label={info.titulo || info.nome}
      sx={{ display: 'flex', flexDirection: 'column', gap: 1.5, p: 1.5, border: 1, borderColor: 'divider', borderRadius: 1 }}
    >
      <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
        {info.nome}
      </Typography>
      <TextField size="small" label="Título" value={info.titulo} disabled={!editavel} onChange={(e) => onMudar({ titulo: e.target.value })} />
      <TextField
        size="small"
        label="Pergunta"
        value={info.pergunta}
        disabled={!editavel}
        multiline
        onChange={(e) => onMudar({ pergunta: e.target.value })}
      />
      <TextField select size="small" label="Tipo" value={tipo} disabled={!editavel} onChange={(e) => onMudar(semOpcoesForaDeEscolha(e.target.value))}>
        {Object.entries(TIPOS_DE_INFORMACAO).map(([valor, rotulo]) => (
          <MenuItem key={valor} value={valor}>
            {rotulo}
          </MenuItem>
        ))}
        {!Object.hasOwn(TIPOS_DE_INFORMACAO, tipo) && <MenuItem value={tipo}>{tipo}</MenuItem>}
      </TextField>
      {tipo === TIPO_ESCOLHA && (
        <ListaEditavel
          titulo="Opções"
          adicionar="Adicionar opção"
          linhas={info.opcoes}
          desabilitado={!editavel}
          criarLinha={() => ({ valor: '', rotulo: '' })}
          onMudar={(opcoes) => onMudar({ opcoes })}
          renderLinha={(opcao, mudar) => (
            <>
              <TextField size="small" label="Valor" value={opcao.valor} disabled={!editavel} onChange={(e) => mudar({ valor: e.target.value })} />
              <TextField size="small" label="Rótulo" value={opcao.rotulo} disabled={!editavel} onChange={(e) => mudar({ rotulo: e.target.value })} />
            </>
          )}
        />
      )}
    </Box>
  );
}

function NovaInformacao({ existentes, onAdicionar }: { existentes: string[]; onAdicionar: (nome: string) => void }): ReactNode {
  const [texto, setTexto] = useState('');
  const nome = nomeDaInformacao(texto);
  const repetida = existentes.includes(nome);
  const adicionar = () => {
    if (nome === '' || repetida) return;
    onAdicionar(nome);
    setTexto('');
  };
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
      <TextField
        size="small"
        fullWidth
        label="Nome da nova informação"
        value={texto}
        error={repetida}
        helperText={repetida ? 'Já existe uma informação com esse nome.' : undefined}
        onChange={(e) => setTexto(e.target.value)}
        onKeyDown={(e) => e.key === 'Enter' && adicionar()}
      />
      <Button startIcon={<AddIcon />} disabled={nome === '' || repetida} onClick={adicionar} sx={{ flexShrink: 0 }}>
        Adicionar informação
      </Button>
    </Box>
  );
}
