import { ListSubheader, MenuItem, TextField } from '@mui/material';
import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { tituloDe } from '@/ferramentas/contratos';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import type { Bloco } from '@/fluxo/modelo/tipos';
import type { CamposProps, CatalogoDeFerramentas } from './camposPorTipo';
import { Texto } from './Texto';

type Campos = CamposProps<'buscar_dados' | 'gravar_no_sistema'>;
type DadoEnviado = Campos['bloco']['com'][number];

const ferramentasDe = (catalogo: CatalogoDeFerramentas): readonly Ferramenta[] =>
  catalogo.estado === 'pronto' ? catalogo.ferramentas : [];

/** Escolher uma ferramenta que grava (ou não) troca o tipo do bloco; fora do catálogo, o tipo fica. */
function comFerramenta(
  bloco: Campos['bloco'],
  catalogo: CatalogoDeFerramentas,
  recurso: string,
): Bloco {
  const escolhida = ferramentasDe(catalogo).find((ferramenta) => ferramenta.name === recurso);
  if (escolhida === undefined) return { ...bloco, recurso };
  return { ...bloco, tipo: escolhida.write ? 'gravar_no_sistema' : 'buscar_dados', recurso };
}

type LinhaProps = {
  linha: DadoEnviado;
  editavel: boolean;
  mudar: (parcial: Partial<DadoEnviado>) => void;
};

function LinhaDeDado({ linha, editavel, mudar }: LinhaProps): ReactNode {
  return (
    <>
      <Texto
        rotulo="Campo"
        valor={linha.campo}
        editavel={editavel}
        onMudar={(campo) => mudar({ campo })}
      />
      {typeof linha.valor === 'string' ? (
        <Texto
          rotulo="Valor"
          valor={linha.valor}
          editavel={editavel}
          onMudar={(valor) => mudar({ valor })}
        />
      ) : (
        <ValorEstruturado valor={linha.valor} />
      )}
    </>
  );
}

export function CamposDeFerramenta({ bloco, editavel, catalogo, onMudar }: Campos): ReactNode {
  return (
    <>
      <SeletorDeFerramenta
        valor={bloco.recurso}
        catalogo={catalogo}
        editavel={editavel}
        onMudar={(recurso) => onMudar(comFerramenta(bloco, catalogo, recurso))}
      />
      <ListaEditavel
        titulo="Dados enviados"
        adicionar="Adicionar dado"
        linhas={bloco.com}
        desabilitado={!editavel}
        criarLinha={() => ({ campo: '', valor: '' })}
        onMudar={(com) => onMudar({ ...bloco, com })}
        renderLinha={(linha, mudar) => (
          <LinhaDeDado linha={linha} editavel={editavel} mudar={mudar} />
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

type SeletorProps = {
  valor: string;
  catalogo: CatalogoDeFerramentas;
  editavel: boolean;
  onMudar: (nome: string) => void;
};

const opcoesQueGravam = (ferramentas: readonly Ferramenta[], write: boolean): ReactNode[] =>
  ferramentas
    .filter((ferramenta) => ferramenta.write === write)
    .map((ferramenta) => (
      <MenuItem key={ferramenta.name} value={ferramenta.name}>
        {tituloDe(ferramenta)}
      </MenuItem>
    ));

function SeletorDeFerramenta({ valor, catalogo, editavel, onMudar }: SeletorProps): ReactNode {
  const ferramentas = ferramentasDe(catalogo);
  const foraDaLista = valor !== '' && !ferramentas.some((ferramenta) => ferramenta.name === valor);
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
      onChange={(evento) => onMudar(evento.target.value)}
    >
      <ListSubheader>Consultas</ListSubheader>
      {opcoesQueGravam(ferramentas, false)}
      <ListSubheader>Gravações</ListSubheader>
      {opcoesQueGravam(ferramentas, true)}
      {foraDaLista && (
        <MenuItem value={valor}>
          {catalogo.estado === 'pronto' ? `${valor} (não encontrada)` : valor}
        </MenuItem>
      )}
    </TextField>
  );
}
