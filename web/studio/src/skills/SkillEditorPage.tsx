import { Box, CircularProgress } from '@mui/material';
import { type ReactNode, useEffect, useRef } from 'react';
import { type Control, Controller, type UseFormSetValue, useForm } from 'react-hook-form';
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import {
  CabecalhoDeDetalhe,
  IconeDaEntidade,
  type PropsDoCabecalho,
} from '@/app/detalhe/CabecalhoDeDetalhe';
import { ESTILO_DO_CONTEUDO_DE_DETALHE } from '@/app/detalhe/LayoutDeDetalhe';
import { PublicarBase } from '@/camadas/PublicarBase';
import { SeletorDeCamada } from '@/camadas/SeletorDeCamada';
import { AvisoDeConflito } from '@/copiloto/AvisoDeConflito';
import { useContextoDaTela } from '@/copiloto/useContextoDaTela';
import { useEdicaoSobOServidor } from '@/copiloto/useEdicaoSobOServidor';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { useValoresDoFormulario } from '@/shared/autosave/useValoresDoFormulario';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { InlineTitle } from '@/shared/ui/InlineTitle';
import { ConhecimentoAba } from './abas/ConhecimentoAba';
import { FerramentasAba } from './abas/FerramentasAba';
import { FluxoAba } from './abas/FluxoAba';
import { RegrasAba } from './abas/RegrasAba';
import { salvarTextos, type Textos, useSkills, useTextos } from './contratos';
import { iconeDaSkill } from './icones';
import { QuandoUsarAba } from './QuandoUsarAba';
import { semVazios, type TextosPendentes, textosPendentesDaRota } from './textosDoEditor';

const ABAS = [
  { id: 'quando-usar', rotulo: 'Quando usar' },
  { id: 'fluxo', rotulo: 'Fluxo' },
  { id: 'regras', rotulo: 'Regras' },
  { id: 'conhecimento', rotulo: 'Base de conhecimento' },
  { id: 'ferramentas', rotulo: 'Ferramentas' },
] as const;
type AbaId = (typeof ABAS)[number]['id'];

const ABA_PADRAO: AbaId = 'quando-usar';

function abaDe(valor: string | null): AbaId {
  return ABAS.find((aba) => aba.id === valor)?.id ?? ABA_PADRAO;
}

export function SkillEditorPage(): ReactNode {
  const { nome = '' } = useParams();
  const lidos = useTextos(nome);
  const { textosPendentes, limparTextosPendentes } = useTextosPendentesDaRota();

  if (lidos.isPending) {
    return (
      <>
        <CabecalhoDaSkill nome={nome} titulo={nome} />
        <Box sx={ESTILO_DO_CONTEUDO_DE_DETALHE}>
          <CircularProgress aria-label="Carregando skill" />
        </Box>
      </>
    );
  }
  if (lidos.isError) {
    return (
      <>
        <CabecalhoDaSkill nome={nome} titulo={nome} />
        <Box sx={ESTILO_DO_CONTEUDO_DE_DETALHE}>
          <ErroComRetry
            mensagem={lidos.error.message}
            onTentarDeNovo={() => void lidos.refetch()}
          />
        </Box>
      </>
    );
  }
  return (
    <Editor
      key={nome}
      nome={nome}
      caminho={lidos.data.caminho}
      textosLidos={lidos.data.textos}
      versaoLida={lidos.data.versao}
      textosPendentes={textosPendentes}
      onTextosPendentesAplicados={limparTextosPendentes}
    />
  );
}

function useTextosPendentesDaRota() {
  const location = useLocation();
  const navegar = useNavigate();
  // Um reload não deve reaplicar os textos pendentes: o estado da rota só vale para esta navegação.
  const limparTextosPendentes = () =>
    navegar({ pathname: location.pathname, search: location.search }, { replace: true });
  return { textosPendentes: textosPendentesDaRota(location.state), limparTextosPendentes };
}

type EditorProps = {
  nome: string;
  caminho: string;
  textosLidos: Textos;
  versaoLida: string;
  textosPendentes: TextosPendentes | undefined;
  onTextosPendentesAplicados: () => void;
};

function Editor(props: EditorProps): ReactNode {
  const { nome, caminho } = props;
  const { control, sincronia, estado, tentarDeNovo } = useTextosDaSkill(props);
  const editavel = usePodeEditar('skills.editar_textos');
  const [params, setParams] = useSearchParams();
  const aba = abaDe(params.get('aba'));
  // Na aba de fluxo, a FluxoAba diz o mesmo contexto: os dois nunca brigam.
  useContextoDaTela({ tela: aba === 'fluxo' ? 'fluxo' : 'skill', skill: nome });

  return (
    <>
      <CabecalhoDaSkill
        nome={nome}
        somenteLeitura={!editavel}
        titulo={<TituloDaSkill control={control} editavel={editavel} />}
        // "Salvo às" vem do cabeçalho; aqui só o que pede atenção (salvando, falha, bloqueio).
        acoes={
          <>
            <SeletorDeCamada skill={nome} />
            <PublicarBase skill={nome} />
            {estado.tipo !== 'salvo' && (
              <SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />
            )}
          </>
        }
        abas={{
          rotulo: 'Abas da skill',
          itens: ABAS,
          atual: aba,
          onTrocar: (proxima) => setParams({ aba: proxima }, { replace: true }),
        }}
      />
      <Box sx={ESTILO_DO_CONTEUDO_DE_DETALHE}>
        <AvisoDeConflito
          conflito={sincronia.conflito}
          onManterOMeu={sincronia.manterOMeu}
          onUsarONovo={sincronia.usarONovo}
        />
        <ConteudoDaAba
          aba={aba}
          nome={nome}
          caminho={caminho}
          control={control}
          editavel={editavel}
        />
      </Box>
    </>
  );
}

type PropsDoCabecalhoDaSkill = Pick<
  PropsDoCabecalho<AbaId>,
  'titulo' | 'acoes' | 'abas' | 'somenteLeitura'
> & {
  nome: string;
};

/** Ícone, tom e categoria vêm do resumo da skill na lista; até chegar, só o título. */
function CabecalhoDaSkill({ nome, ...props }: PropsDoCabecalhoDaSkill): ReactNode {
  const resumo = useSkills().data?.skills.find((skill) => skill.nome === nome);
  const categoria = resumo?.escrita ? 'registra no sistema' : 'consulta e dúvida';
  return (
    <CabecalhoDeDetalhe
      voltar={{ para: '/skills', rotulo: 'Voltar para Skills' }}
      icone={
        resumo && (
          <IconeDaEntidade
            Icone={iconeDaSkill(resumo.icone)}
            tom={resumo.escrita ? 'registra' : 'consulta'}
          />
        )
      }
      subtitulo={resumo && `Skills · ${categoria}`}
      testarNoConsole
      {...props}
    />
  );
}

function useTextosDaSkill(props: EditorProps) {
  const { nome, textosLidos, versaoLida } = props;
  const { control, setValue, reset } = useForm({ defaultValues: textosLidos });
  const textos = useValoresDoFormulario(control);
  useAplicarTextosPendentes(setValue, props);

  const sincronia = useEdicaoSobOServidor({
    servidor: textosLidos,
    alvos: [`skill:${nome}`],
    versao: versaoLida,
    local: textos,
    resetar: reset,
    normalizar: semVazios,
    salvar: async (valor, versao) =>
      (await salvarTextos(nome, { versao, textos: semVazios(valor) })).versao,
  });
  const { estado, tentarDeNovo } = useAutosave({
    valor: textos,
    salvar: sincronia.salvar,
    bloqueio: sincronia.bloqueio,
  });
  return { control, sincronia, estado, tentarDeNovo };
}

function useAplicarTextosPendentes(
  setValue: UseFormSetValue<Textos>,
  { textosPendentes, onTextosPendentesAplicados }: EditorProps,
): void {
  const aplicou = useRef(false);
  // Entram como alteração pendente (shouldDirty) sobre o que veio do servidor, não como base já salva:
  // o autosave só manda ao servidor o que ele vê como diferente do que carregou.
  useEffect(() => {
    if (aplicou.current || !textosPendentes) return;
    aplicou.current = true;
    setValue('resumo', textosPendentes.resumo, { shouldDirty: true });
    setValue('quandoUsar', textosPendentes.quandoUsar, { shouldDirty: true });
    onTextosPendentesAplicados();
  }, [textosPendentes, setValue, onTextosPendentesAplicados]);
}

function TituloDaSkill({ control, editavel }: { control: Control<Textos>; editavel: boolean }) {
  return (
    <Controller
      control={control}
      name="titulo"
      render={({ field }) => (
        <InlineTitle
          component="span"
          aparencia="herdada"
          rotulo="Nome da skill"
          valor={field.value}
          editavel={editavel}
          onMudar={field.onChange}
        />
      )}
    />
  );
}

type ConteudoProps = {
  aba: AbaId;
  nome: string;
  caminho: string;
  control: Control<Textos>;
  editavel: boolean;
};

function ConteudoDaAba({ aba, nome, caminho, control, editavel }: ConteudoProps): ReactNode {
  switch (aba) {
    case 'quando-usar':
      return <QuandoUsarAba nome={nome} control={control} editavel={editavel} />;
    case 'fluxo':
      return <FluxoAba nome={nome} caminho={caminho} />;
    case 'regras':
      return <RegrasAba nome={nome} caminho={caminho} />;
    case 'conhecimento':
      return <ConhecimentoAba nome={nome} caminho={caminho} />;
    case 'ferramentas':
      return <FerramentasAba nome={nome} caminho={caminho} />;
    default:
      return assertNever(aba);
  }
}

function assertNever(valor: never): never {
  throw new Error(`aba não tratada: ${JSON.stringify(valor)}`);
}
