import { zodResolver } from '@hookform/resolvers/zod';
import AddIcon from '@mui/icons-material/Add';
import { Box, Button, MenuItem, TextField, Typography } from '@mui/material';
import type { ReactNode } from 'react';
import { useForm, useWatch } from 'react-hook-form';
import { z } from 'zod';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import { type Informacao, InformacaoSchema } from '@/fluxo/modelo/tipos';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { paraIdentificador } from '@/skills/contratos';

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
        <CartaoDeInformacao
          key={info.nome}
          info={info}
          editavel={editavel}
          onMudar={(parcial) => mudar(info.nome, parcial)}
        />
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

type CartaoProps = {
  info: Informacao;
  editavel: boolean;
  onMudar: (parcial: Partial<Informacao>) => void;
};

function CartaoDeInformacao({ info, editavel, onMudar }: CartaoProps): ReactNode {
  const tipo = tipoMostrado(info.tipo);
  return (
    <Box
      component="article"
      aria-label={info.titulo || info.nome}
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
      <Typography variant="caption" color="text.secondary" sx={{ fontFamily: 'monospace' }}>
        {info.nome}
      </Typography>
      <TextField
        size="small"
        label="Título"
        value={info.titulo}
        disabled={!editavel}
        onChange={(e) => onMudar({ titulo: e.target.value })}
      />
      <TextField
        size="small"
        label="Pergunta"
        value={info.pergunta}
        disabled={!editavel}
        multiline
        onChange={(e) => onMudar({ pergunta: e.target.value })}
      />
      <TextField
        select
        size="small"
        label="Tipo"
        value={tipo}
        disabled={!editavel}
        onChange={(e) => onMudar(semOpcoesForaDeEscolha(e.target.value))}
      >
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
              <TextField
                size="small"
                label="Valor"
                value={opcao.valor}
                disabled={!editavel}
                onChange={(e) => mudar({ valor: e.target.value })}
              />
              <TextField
                size="small"
                label="Rótulo"
                value={opcao.rotulo}
                disabled={!editavel}
                onChange={(e) => mudar({ rotulo: e.target.value })}
              />
            </>
          )}
        />
      )}
    </Box>
  );
}

const esquemaDaNova = (existentes: string[]) =>
  z.object({
    texto: z
      .string()
      .refine(
        (texto) => !existentes.includes(nomeDaInformacao(texto)),
        'Já existe uma informação com esse nome.',
      ),
  });

function NovaInformacao({
  existentes,
  onAdicionar,
}: {
  existentes: string[];
  onAdicionar: (nome: string) => void;
}): ReactNode {
  // onChange: a repetição aparece enquanto se digita, não só ao tentar adicionar.
  const { control, handleSubmit, reset } = useForm({
    mode: 'onChange',
    resolver: zodResolver(esquemaDaNova(existentes)),
    defaultValues: { texto: '' },
  });
  const nome = nomeDaInformacao(useWatch({ control, name: 'texto' }));
  const repetida = existentes.includes(nome);
  const adicionar = handleSubmit(({ texto }) => {
    if (nomeDaInformacao(texto) === '') return;
    onAdicionar(nomeDaInformacao(texto));
    reset();
  });
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
      <CampoTexto
        control={control}
        name="texto"
        size="small"
        fullWidth
        label="Nome da nova informação"
        onKeyDown={(e) => e.key === 'Enter' && void adicionar()}
      />
      <Button
        startIcon={<AddIcon />}
        disabled={nome === '' || repetida}
        onClick={() => void adicionar()}
        sx={{ flexShrink: 0 }}
      >
        Adicionar informação
      </Button>
    </Box>
  );
}
