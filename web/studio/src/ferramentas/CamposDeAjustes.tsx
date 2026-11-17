import type { ReactNode } from 'react';
import { type Control, Controller } from 'react-hook-form';
import { SeletorDaLinha, TextoDaLinha } from './CamposDaLinha';
import { padraoDoAjuste, TIPOS_DE_AJUSTE } from './contratos';
import type { RecursoNoFormulario, Valores } from './formularioDaFerramenta';
import { ListaEditavel } from './ListaEditavel';

type Props = { control: Control<Valores>; travado: boolean };
type Ajuste = RecursoNoFormulario['definicoes'][number];

const TIPOS = Object.entries(TIPOS_DE_AJUSTE);

const textoDoPadrao = (valor: unknown): string =>
  valor === undefined || valor === null ? '' : String(valor);

const novoAjuste = (): Ajuste => ({ name: '', label: '', type: 'string', options: [] });

type LinhaProps = {
  ajuste: Ajuste;
  travado: boolean;
  mudar: (parcial: Partial<Ajuste>) => void;
};

function LinhaDeAjuste({ ajuste, travado, mudar }: LinhaProps): ReactNode {
  return (
    <>
      <TextoDaLinha
        rotulo="Nome do ajuste"
        dica="prazo_dias"
        valor={ajuste.name}
        travado={travado}
        onMudar={(name) => mudar({ name })}
      />
      <TextoDaLinha
        rotulo="Rótulo do ajuste"
        dica="Como aparece para o condomínio"
        valor={ajuste.label}
        travado={travado}
        onMudar={(label) => mudar({ label })}
      />
      <SeletorDaLinha
        rotulo="Tipo do ajuste"
        valor={ajuste.type ?? 'string'}
        opcoes={TIPOS}
        travado={travado}
        onMudar={(type) => mudar({ type })}
      />
      <TextoDaLinha
        rotulo="Padrão do ajuste"
        dica="Valor quando o condomínio não decide"
        valor={textoDoPadrao(ajuste.default)}
        travado={travado}
        onMudar={(texto) => mudar({ default: padraoDoAjuste(ajuste.type, texto) })}
      />
      {ajuste.type === 'select' && (
        <TextoDaLinha
          rotulo="Opções do ajuste"
          dica="Opções, separadas por vírgula"
          valor={ajuste.options.join(',')}
          travado={travado}
          onMudar={(texto) => mudar({ options: texto.split(',') })}
        />
      )}
      <TextoDaLinha
        rotulo="Descrição do ajuste"
        dica="Para que serve"
        valor={ajuste.description ?? ''}
        travado={travado}
        larguraTotal
        onMudar={(description) => mudar({ description })}
      />
    </>
  );
}

export function CamposDeAjustes({ control, travado }: Props): ReactNode {
  return (
    <Controller
      control={control}
      name="recurso.definicoes"
      render={({ field }) => (
        <ListaEditavel<Ajuste>
          titulo="Ajustes por condomínio"
          adicionar="Adicionar ajuste"
          linhas={field.value}
          desabilitado={travado}
          criarLinha={novoAjuste}
          onMudar={field.onChange}
          renderLinha={(ajuste, mudar) => (
            <LinhaDeAjuste ajuste={ajuste} travado={travado} mudar={mudar} />
          )}
        />
      )}
    />
  );
}
