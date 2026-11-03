import { Alert, Box, Button, CircularProgress, Tab, Tabs } from '@mui/material';
import { useRef, useState, type ReactNode } from 'react';
import { useParams, useSearchParams } from 'react-router';
import { usePodeEditar } from '@/acesso/eu';
import { SaveIndicator } from '@/shared/autosave/SaveIndicator';
import { useAutosave } from '@/shared/autosave/useAutosave';
import { InlineTitle } from '@/shared/ui/InlineTitle';
import { ConhecimentoAba } from './abas/ConhecimentoAba';
import { FerramentasAba } from './abas/FerramentasAba';
import { FluxoAba } from './abas/FluxoAba';
import { RegrasAba } from './abas/RegrasAba';
import { salvarTextos, useTextos, type Textos } from './contratos';
import { QuandoUsarAba } from './QuandoUsarAba';

const ABAS = [
  { id: 'quando-usar', rotulo: 'Quando usar' },
  { id: 'fluxo', rotulo: 'Fluxo' },
  { id: 'regras', rotulo: 'Regras' },
  { id: 'conhecimento', rotulo: 'Base de conhecimento' },
  { id: 'ferramentas', rotulo: 'Ferramentas' },
] as const;
type AbaId = (typeof ABAS)[number]['id'];

const ABA_PADRAO: AbaId = 'quando-usar';

const abaDe = (valor: string | null): AbaId => ABAS.find((aba) => aba.id === valor)?.id ?? ABA_PADRAO;

const aparadasSemVazias = (itens: string[]): string[] => itens.map((i) => i.trim()).filter((i) => i !== '');

/** Frase ou palavra em branco é rascunho na tela, não vai para o servidor. */
const semVazios = (textos: Textos): Textos => ({
  ...textos,
  palavrasChave: aparadasSemVazias(textos.palavrasChave),
  frasesDeExemplo: aparadasSemVazias(textos.frasesDeExemplo),
});

export function SkillEditorPage(): ReactNode {
  const { nome = '' } = useParams();
  const lidos = useTextos(nome);

  if (lidos.isPending) return <CircularProgress aria-label="Carregando skill" />;
  if (lidos.isError) {
    return (
      <Alert severity="error" action={<Button onClick={() => void lidos.refetch()}>Tentar de novo</Button>}>
        {lidos.error.message}
      </Alert>
    );
  }
  return (
    <Editor
      key={nome}
      nome={nome}
      caminho={lidos.data.caminho}
      textosLidos={lidos.data.textos}
      versaoLida={lidos.data.versao}
    />
  );
}

type EditorProps = { nome: string; caminho: string; textosLidos: Textos; versaoLida: string };

function Editor({ nome, caminho, textosLidos, versaoLida }: EditorProps): ReactNode {
  const [textos, setTextos] = useState(textosLidos);
  const versao = useRef(versaoLida);
  const editavel = usePodeEditar('skills.editar_textos');
  const [params, setParams] = useSearchParams();
  const aba = abaDe(params.get('aba'));

  const { estado, tentarDeNovo } = useAutosave({
    valor: textos,
    salvar: async (valor) => {
      const gravado = await salvarTextos(nome, { versao: versao.current, textos: semVazios(valor) });
      versao.current = gravado.versao;
    },
  });

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
        <InlineTitle
          rotulo="Nome da skill"
          valor={textos.titulo}
          editavel={editavel}
          onMudar={(titulo) => setTextos({ ...textos, titulo })}
        />
        <SaveIndicator estado={estado} onTentarDeNovo={() => void tentarDeNovo()} />
      </Box>
      <Tabs
        value={aba}
        onChange={(_, proxima: AbaId) => setParams({ aba: proxima }, { replace: true })}
        variant="scrollable"
      >
        {ABAS.map(({ id, rotulo }) => (
          <Tab key={id} value={id} label={rotulo} />
        ))}
      </Tabs>
      <ConteudoDaAba aba={aba} nome={nome} caminho={caminho} textos={textos} editavel={editavel} onMudar={setTextos} />
    </Box>
  );
}

type ConteudoProps = {
  aba: AbaId;
  nome: string;
  caminho: string;
  textos: Textos;
  editavel: boolean;
  onMudar: (textos: Textos) => void;
};

function ConteudoDaAba({ aba, nome, caminho, textos, editavel, onMudar }: ConteudoProps): ReactNode {
  switch (aba) {
    case 'quando-usar':
      return <QuandoUsarAba textos={textos} editavel={editavel} onMudar={onMudar} />;
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
