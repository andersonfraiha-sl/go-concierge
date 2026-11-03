import {
  FormControl,
  FormControlLabel,
  InputLabel,
  ListSubheader,
  MenuItem,
  OutlinedInput,
  Select,
  Switch,
  TextField,
} from '@mui/material';
import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import { tituloDe } from '@/ferramentas/contratos';
import type { Bloco, Informacao, TipoBloco } from '@/fluxo/modelo/tipos';

type BlocoDo<T extends TipoBloco> = Extract<Bloco, { tipo: T }>;

type Contexto = {
  editavel: boolean;
  ferramentas: readonly Ferramenta[];
  informacoes: readonly Informacao[];
  /** Recebe `Bloco`, não o tipo exato: escolher a ferramenta pode trocar Buscar por Gravar. */
  onMudar: (bloco: Bloco) => void;
};

type CamposProps<T extends TipoBloco> = Contexto & { bloco: BlocoDo<T> };

const CAMPOS: { [T in TipoBloco]: (props: CamposProps<T>) => ReactNode } = {
  buscar_dados: CamposDeFerramenta,
  gravar_no_sistema: CamposDeFerramenta,
  pedir_escolha: CamposDePedirEscolha,
  perguntar: ({ bloco, editavel, informacoes, onMudar }) => (
    <SelecaoDeInformacoes
      rotulo="Informações perguntadas"
      valor={bloco.campos}
      informacoes={informacoes}
      editavel={editavel}
      onMudar={(campos) => onMudar({ ...bloco, campos })}
    />
  ),
  formulario: ({ bloco, editavel, onMudar }) => (
    <>
      <Texto rotulo="Campos vêm de" valor={bloco.de} editavel={editavel} onMudar={(de) => onMudar({ ...bloco, de })} />
      <Texto
        rotulo="Guardar respostas em"
        valor={bloco.guardarEm}
        editavel={editavel}
        onMudar={(guardarEm) => onMudar({ ...bloco, guardarEm })}
      />
    </>
  ),
  confirmar: ({ bloco, editavel, onMudar }) => (
    <Texto
      rotulo="Pergunta de confirmação"
      valor={bloco.texto}
      editavel={editavel}
      linhas={3}
      onMudar={(texto) => onMudar({ ...bloco, texto })}
    />
  ),
  montar_texto: ({ bloco, editavel, onMudar }) => (
    <Texto
      rotulo="Instrução para a IA"
      valor={bloco.texto}
      editavel={editavel}
      linhas={5}
      onMudar={(texto) => onMudar({ ...bloco, texto })}
    />
  ),
  reaproveitar: ({ bloco, editavel, informacoes, onMudar }) => (
    <>
      <SelecaoDeInformacoes
        rotulo="Informações reaproveitadas"
        valor={bloco.campos}
        informacoes={informacoes}
        editavel={editavel}
        onMudar={(campos) => onMudar({ ...bloco, campos })}
      />
      <Texto
        rotulo="Guardar a resposta em"
        valor={bloco.confirmacao}
        editavel={editavel}
        onMudar={(confirmacao) => onMudar({ ...bloco, confirmacao })}
      />
      <Texto rotulo="Pergunta" valor={bloco.pergunta} editavel={editavel} onMudar={(pergunta) => onMudar({ ...bloco, pergunta })} />
    </>
  ),
  responder: ({ bloco, editavel, onMudar }) => (
    <Texto rotulo="Resposta" valor={bloco.texto} editavel={editavel} linhas={5} onMudar={(texto) => onMudar({ ...bloco, texto })} />
  ),
};

/** Campos do bloco conforme o tipo. O que o Studio não edita (`extra` e afins) volta intacto pelo spread. */
export function CamposDoBloco(props: CamposProps<TipoBloco>): ReactNode {
  return renderizar(props.bloco.tipo, props);
}

function renderizar<T extends TipoBloco>(tipo: T, props: CamposProps<T>): ReactNode {
  const Campos: (props: CamposProps<T>) => ReactNode = CAMPOS[tipo];
  return <Campos {...props} />;
}

type TextoProps = { rotulo: string; valor: string; editavel: boolean; linhas?: number; onMudar: (valor: string) => void };

function Texto({ rotulo, valor, editavel, linhas, onMudar }: TextoProps): ReactNode {
  return (
    <TextField
      size="small"
      fullWidth
      label={rotulo}
      value={valor}
      disabled={!editavel}
      multiline={linhas !== undefined}
      minRows={linhas}
      onChange={(evento) => onMudar(evento.target.value)}
    />
  );
}

function CamposDeFerramenta({ bloco, editavel, ferramentas, onMudar }: CamposProps<'buscar_dados' | 'gravar_no_sistema'>): ReactNode {
  const escolher = (recurso: string) => {
    const escolhida = ferramentas.find((f) => f.name === recurso);
    const tipo = escolhida === undefined ? bloco.tipo : escolhida.write ? 'gravar_no_sistema' : 'buscar_dados';
    onMudar({ ...bloco, tipo, recurso });
  };
  return (
    <>
      <SeletorDeFerramenta valor={bloco.recurso} ferramentas={ferramentas} editavel={editavel} onMudar={escolher} />
      <ListaEditavel
        titulo="Dados enviados"
        adicionar="Adicionar dado"
        linhas={bloco.com}
        desabilitado={!editavel}
        criarLinha={() => ({ campo: '', valor: '' })}
        onMudar={(com) => onMudar({ ...bloco, com })}
        renderLinha={(linha, mudar) => (
          <>
            <Texto rotulo="Campo" valor={linha.campo} editavel={editavel} onMudar={(campo) => mudar({ campo })} />
            <Texto rotulo="Valor" valor={textoDoValor(linha.valor)} editavel={editavel} onMudar={(valor) => mudar({ valor })} />
          </>
        )}
      />
      <Texto
        rotulo="Guardar resultado em"
        valor={bloco.guardarEm}
        editavel={editavel}
        onMudar={(guardarEm) => onMudar({ ...bloco, guardarEm })}
      />
    </>
  );
}

// ponytail: valor que não é texto aparece como JSON e, se editado, volta como texto. Editor de JSON se aparecer caso real.
const textoDoValor = (valor: unknown): string => (typeof valor === 'string' ? valor : JSON.stringify(valor));

type SeletorProps = { valor: string; ferramentas: readonly Ferramenta[]; editavel: boolean; onMudar: (nome: string) => void };

function SeletorDeFerramenta({ valor, ferramentas, editavel, onMudar }: SeletorProps): ReactNode {
  const opcoes = (write: boolean) =>
    ferramentas
      .filter((f) => f.write === write)
      .map((f) => (
        <MenuItem key={f.name} value={f.name}>
          {tituloDe(f)}
        </MenuItem>
      ));
  const desconhecida = valor !== '' && !ferramentas.some((f) => f.name === valor);
  return (
    <TextField select size="small" fullWidth label="Ferramenta" value={valor} disabled={!editavel} onChange={(e) => onMudar(e.target.value)}>
      <ListSubheader>Consultas</ListSubheader>
      {opcoes(false)}
      <ListSubheader>Gravações</ListSubheader>
      {opcoes(true)}
      {desconhecida && (
        <MenuItem value={valor}>
          {valor} (não encontrada)
        </MenuItem>
      )}
    </TextField>
  );
}

function CamposDePedirEscolha({ bloco, editavel, onMudar }: CamposProps<'pedir_escolha'>): ReactNode {
  const texto = (rotulo: string, chave: 'param' | 'titulo' | 'de' | 'rotulo' | 'valor' | 'pergunta' | 'seVazio') => (
    <Texto rotulo={rotulo} valor={bloco[chave]} editavel={editavel} onMudar={(valor) => onMudar({ ...bloco, [chave]: valor })} />
  );
  return (
    <>
      {texto('Guardar escolha em', 'param')}
      {texto('Título', 'titulo')}
      {texto('Opções vêm de', 'de')}
      {texto('Mostrar o campo', 'rotulo')}
      {texto('Usar o valor do campo', 'valor')}
      {texto('Pergunta', 'pergunta')}
      {texto('Se não houver opções', 'seVazio')}
      <FormControlLabel
        label="Vale para a conversa toda"
        disabled={!editavel}
        control={
          <Switch
            checked={bloco.reaproveitar === 'always'}
            onChange={(_, ligado) => onMudar({ ...bloco, reaproveitar: ligado ? 'always' : '' })}
          />
        }
      />
    </>
  );
}

type SelecaoProps = {
  rotulo: string;
  valor: readonly string[];
  informacoes: readonly Informacao[];
  editavel: boolean;
  onMudar: (nomes: string[]) => void;
};

function SelecaoDeInformacoes({ rotulo, valor, informacoes, editavel, onMudar }: SelecaoProps): ReactNode {
  const titulo = (nome: string): string => informacoes.find((i) => i.nome === nome)?.titulo || nome;
  const semCadastro = valor.filter((nome) => !informacoes.some((i) => i.nome === nome));
  const nomes = [...informacoes.map((i) => i.nome), ...semCadastro];
  const id = `selecao-${rotulo}`;
  return (
    <FormControl size="small" fullWidth disabled={!editavel}>
      <InputLabel id={id}>{rotulo}</InputLabel>
      <Select<string[]>
        multiple
        labelId={id}
        value={[...valor]}
        input={<OutlinedInput label={rotulo} />}
        renderValue={(escolhidos) => escolhidos.map(titulo).join(', ')}
        onChange={(evento) => {
          const escolhidos = evento.target.value;
          onMudar(typeof escolhidos === 'string' ? escolhidos.split(',') : escolhidos);
        }}
      >
        {nomes.map((nome) => (
          <MenuItem key={nome} value={nome}>
            {titulo(nome)}
          </MenuItem>
        ))}
      </Select>
    </FormControl>
  );
}
