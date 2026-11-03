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
import { useId, type ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import { tituloDe } from '@/ferramentas/contratos';
import type { Bloco, Informacao, TipoBloco } from '@/fluxo/modelo/tipos';

type BlocoDo<T extends TipoBloco> = Extract<Bloco, { tipo: T }>;

/** O que se sabe do catálogo de ferramentas; enquanto não está pronto, nenhuma é "não encontrada". */
export type CatalogoDeFerramentas =
  | { estado: 'carregando' }
  | { estado: 'erro' }
  | { estado: 'pronto'; ferramentas: readonly Ferramenta[] };

type Contexto = {
  editavel: boolean;
  catalogo: CatalogoDeFerramentas;
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

const ferramentasDe = (catalogo: CatalogoDeFerramentas): readonly Ferramenta[] =>
  catalogo.estado === 'pronto' ? catalogo.ferramentas : [];

function CamposDeFerramenta({ bloco, editavel, catalogo, onMudar }: CamposProps<'buscar_dados' | 'gravar_no_sistema'>): ReactNode {
  const escolher = (recurso: string) => {
    const escolhida = ferramentasDe(catalogo).find((f) => f.name === recurso);
    const tipo = escolhida === undefined ? bloco.tipo : escolhida.write ? 'gravar_no_sistema' : 'buscar_dados';
    onMudar({ ...bloco, tipo, recurso });
  };
  return (
    <>
      <SeletorDeFerramenta valor={bloco.recurso} catalogo={catalogo} editavel={editavel} onMudar={escolher} />
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
            {typeof linha.valor === 'string' ? (
              <Texto rotulo="Valor" valor={linha.valor} editavel={editavel} onMudar={(valor) => mudar({ valor })} />
            ) : (
              <ValorEstruturado valor={linha.valor} />
            )}
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

/** Lista ou objeto não cabe num campo de texto: editar aqui gravaria texto no lugar da estrutura. */
function ValorEstruturado({ valor }: { valor: unknown }): ReactNode {
  return (
    <TextField
      size="small"
      fullWidth
      label="Valor"
      value={JSON.stringify(valor)}
      disabled
      helperText="Este valor só pode ser editado na área Avançado."
    />
  );
}

const AVISO_DO_CATALOGO: Record<CatalogoDeFerramentas['estado'], string | undefined> = {
  carregando: 'Carregando ferramentas…',
  erro: 'Não deu para carregar as ferramentas.',
  pronto: undefined,
};

type SeletorProps = { valor: string; catalogo: CatalogoDeFerramentas; editavel: boolean; onMudar: (nome: string) => void };

function SeletorDeFerramenta({ valor, catalogo, editavel, onMudar }: SeletorProps): ReactNode {
  const ferramentas = ferramentasDe(catalogo);
  const opcoes = (write: boolean) =>
    ferramentas
      .filter((f) => f.write === write)
      .map((f) => (
        <MenuItem key={f.name} value={f.name}>
          {tituloDe(f)}
        </MenuItem>
      ));
  const foraDaLista = valor !== '' && !ferramentas.some((f) => f.name === valor);
  return (
    <TextField
      select
      size="small"
      fullWidth
      label="Ferramenta"
      value={valor}
      disabled={!editavel}
      error={catalogo.estado === 'erro'}
      helperText={AVISO_DO_CATALOGO[catalogo.estado]}
      onChange={(e) => onMudar(e.target.value)}
    >
      <ListSubheader>Consultas</ListSubheader>
      {opcoes(false)}
      <ListSubheader>Gravações</ListSubheader>
      {opcoes(true)}
      {foraDaLista && (
        <MenuItem value={valor}>{catalogo.estado === 'pronto' ? `${valor} (não encontrada)` : valor}</MenuItem>
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
  const id = useId();
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
