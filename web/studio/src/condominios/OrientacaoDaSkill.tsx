import { CircularProgress } from '@mui/material';
import type { ReactNode } from 'react';
import { usePodeEditarCondominio } from '@/acesso/eu';
import { type Camada, type CamadaLida, useCamada } from '@/camadas/contratos';
import { PublicarCamada } from '@/camadas/PublicarCamada';
import { useRascunhoDaCamada } from '@/camadas/useRascunhoDaCamada';
import { useSnackbar } from '@/shared/snackbar/SnackbarContext';
import { CampoTexto } from '@/shared/ui/CampoTexto';
import { ErroComRetry } from '@/shared/ui/ErroComRetry';
import { type Caso, type Estado, PADRAO } from './contratos';
import { exemploDeOrientacao } from './listaDeSkills';
import { OrientacaoDoPadrao } from './OrientacaoDoPadrao';
import { SecaoComSalvamento } from './SecaoComSalvamento';

type Props = { estado: Estado; condominio: string; caso: Caso; editavel: boolean };

const TITULO = 'Orientação desta skill';

/**
 * A orientação do condomínio mora na camada dele. O Padrão não tem camada (vale para todos os
 * condomínios): segue na orientação antiga.
 */
export function OrientacaoDaSkill(props: Props): ReactNode {
  if (props.condominio === PADRAO) return <OrientacaoDoPadrao {...props} />;
  const camada: Camada = { skill: props.caso.nome, tipo: 'condominio', id: props.condominio };
  return <OrientacaoNaCamada camada={camada} caso={props.caso} editavel={props.editavel} />;
}

type PropsDaCamada = { camada: Camada; caso: Caso; editavel: boolean };

function OrientacaoNaCamada({ camada, caso, editavel }: PropsDaCamada): ReactNode {
  const lida = useCamada(camada.skill, camada.tipo, camada.id);
  if (lida.isPending) return <CircularProgress aria-label="Carregando orientação" />;
  if (lida.isError) {
    return (
      <ErroComRetry mensagem={lida.error.message} onTentarDeNovo={() => void lida.refetch()} />
    );
  }
  return <EditorDaOrientacao camada={camada} caso={caso} editavel={editavel} lida={lida.data} />;
}

function EditorDaOrientacao({
  camada,
  caso,
  editavel,
  lida,
}: PropsDaCamada & { lida: CamadaLida }): ReactNode {
  const rascunho = useRascunhoDaCamada(camada, lida);
  const { mostrar } = useSnackbar();
  // Gravar o rascunho com orientação nova pede editar etapas; publicar pede editar textos.
  const podeEditarEtapas = usePodeEditarCondominio('skills.editar_etapas');
  const podeEditarTextos = usePodeEditarCondominio('skills.editar_textos');
  const podeEscrever = editavel && podeEditarEtapas;
  const podePublicar = podeEscrever && podeEditarTextos;
  const ocupado = rascunho.salvamento.tipo === 'salvando' || rascunho.salvamento.tipo === 'erro';

  return (
    <SecaoComSalvamento
      titulo={TITULO}
      salvamento={rascunho.salvamento}
      tentarDeNovo={rascunho.tentarDeNovo}
    >
      <CampoTexto
        control={rascunho.controlDaOrientacao}
        name="orientacao"
        multiline
        minRows={3}
        fullWidth
        disabled={!podeEscrever}
        placeholder={`Ex.: ${exemploDeOrientacao(caso.nome)}`}
        slotProps={{ htmlInput: { 'aria-label': TITULO } }}
      />
      {lida.temRascunho && podePublicar && (
        <PublicarCamada
          desabilitado={ocupado}
          onPublicar={async (resumo) => {
            await rascunho.publicar(resumo);
            mostrar('Orientação publicada.');
          }}
        />
      )}
    </SecaoComSalvamento>
  );
}
