import { ListSubheader, MenuItem, TextField } from '@mui/material';
import type { ReactNode } from 'react';
import type { Ferramenta } from '@/catalogo/contratos';
import { tituloDe } from '@/ferramentas/contratos';
import { ListaEditavel } from '@/ferramentas/ListaEditavel';
import type { CamposProps, CatalogoDeFerramentas } from './camposPorTipo';
import { Texto } from './Texto';

const ferramentasDe = (catalogo: CatalogoDeFerramentas): readonly Ferramenta[] =>
  catalogo.estado === 'pronto' ? catalogo.ferramentas : [];

export function CamposDeFerramenta({
  bloco,
  editavel,
  catalogo,
  onMudar,
}: CamposProps<'buscar_dados' | 'gravar_no_sistema'>): ReactNode {
  const escolher = (recurso: string) => {
    const escolhida = ferramentasDe(catalogo).find((f) => f.name === recurso);
    const tipo =
      escolhida === undefined ? bloco.tipo : escolhida.write ? 'gravar_no_sistema' : 'buscar_dados';
    onMudar({ ...bloco, tipo, recurso });
  };
  return (
    <>
      <SeletorDeFerramenta
        valor={bloco.recurso}
        catalogo={catalogo}
        editavel={editavel}
        onMudar={escolher}
      />
      <ListaEditavel
        titulo="Dados enviados"
        adicionar="Adicionar dado"
        linhas={bloco.com}
        desabilitado={!editavel}
        criarLinha={() => ({ campo: '', valor: '' })}
        onMudar={(com) => onMudar({ ...bloco, com })}
        renderLinha={(linha, mudar) => (
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
        <MenuItem value={valor}>
          {catalogo.estado === 'pronto' ? `${valor} (não encontrada)` : valor}
        </MenuItem>
      )}
    </TextField>
  );
}
