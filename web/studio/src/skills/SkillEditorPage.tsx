import { Box, CircularProgress, Tab, Tabs } from '@mui/material';
import { type ReactNode, useEffect, useRef } from 'react';
import { type Control, Controller, useForm } from 'react-hook-form';
import { useLocation, useNavigate, useParams, useSearchParams } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { AvisoDoCopiloto } from '@/copiloto/AvisoDoCopiloto';
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
import { salvarTextos, type Textos, useTextos } from './contratos';
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

const abaDe = (valor: string | null): AbaId =>
  ABAS.find((aba) => aba.id === valor)?.id ?? ABA_PADRAO;

export function SkillEditorPage(): ReactNode {
  const { nome = '' } = useParams();
  const lidos = useTextos(nome);
  const location = useLocation();
  const navegar = useNavigate();
  const textosPendentes = textosPendentesDaRota(location.state);

  // Um reload não deve reaplicar os textos pendentes: o estado da rota só vale para esta navegação.
  const limparTextosPendentes = () =>
    navegar({ pathname: location.pathname, search: location.search }, { replace: true });

  if (lidos.isPending) return <CircularProgress aria-label="Carregando skill" />;
  if (lidos.isError) {
    return (
      <ErroComRetry mensagem={lidos.error.message} onTentarDeNovo={() => void lidos.refetch()} />
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

type EditorProps = {
  nome: string;
  caminho: string;
  textosLidos: Textos;
  versaoLida: string;
  textosPendentes: TextosPendentes | undefined;
  onTextosPendentesAplicados: () => void;
};

function Editor({
  nome,
  caminho,
  textosLidos,
  versaoLida,
  textosPendentes,
  onTextosPendentesAplicados,
}: EditorProps): ReactNode {
  const { control, setValue, reset } = useForm({ defaultValues: textosLidos });
  const textos = useValoresDoFormulario(control);
  const versao = useRef(versaoLida);
  const aplicouTextosPendentes = useRef(false);
  const editavel = usePodeEditar('skills.editar_textos');
  const [params, setParams] = useSearchParams();
  const aba = abaDe(params.get('aba'));
  useContextoDaTela(aba === 'fluxo' ? null : { tela: 'skill', skill: nome });

  // Entram como alteração pendente (shouldDirty) sobre o que veio do servidor, não como base já salva:
  // o autosave só manda ao servidor o que ele vê como diferente do que carregou.
  useEffect(() => {
    if (aplicouTextosPendentes.current || !textosPendentes) return;
    aplicouTextosPendentes.current = true;
    setValue('resumo', textosPendentes.resumo, { shouldDirty: true });
    setValue('quandoUsar', textosPendentes.quandoUsar, { shouldDirty: true });
    onTextosPendentesAplicados();
  }, [textosPendentes, setValue, onTextosPendentesAplicados]);

  // Uma proposta do Copiloto aplicada traz textos e versão novos do servidor.
  useEffect(() => {
    versao.current = versaoLida;
  }, [versaoLida]);
  const sincronia = useEdicaoSobOServidor({
    servidor: textosLidos,
    local: textos,
    resetar: reset,
    salvar: async (valor) => {
      const gravado = await salvarTextos(nome, {
        versao: versao.current,
        textos: semVazios(valor),
      });
      versao.current = gravado.versao;
    },
  });
  const { estado, tentarDeNovo } = useAutosave({ valor: textos, salvar: sincronia.salvar });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        <Controller
          control={control}
          name="titulo"
          render={({ field }) => (
            <InlineTitle
              rotulo="Nome da skill"
              valor={field.value}
              editavel={editavel}
              onMudar={field.onChange}
            />
          )}
        />
        <SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />
      </Box>
      <AvisoDoCopiloto visivel={sincronia.mudouSobEdicao} onFechar={sincronia.dispensarAviso} />
      <Tabs
        value={aba}
        onChange={(_, proxima: AbaId) => setParams({ aba: proxima }, { replace: true })}
        variant="scrollable"
      >
        {ABAS.map(({ id, rotulo }) => (
          <Tab key={id} value={id} label={rotulo} />
        ))}
      </Tabs>
      <ConteudoDaAba
        aba={aba}
        nome={nome}
        caminho={caminho}
        control={control}
        editavel={editavel}
      />
    </Box>
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
