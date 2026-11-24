import { createContext, useContext } from 'react';

type Registrar = (em: Date) => void;

const naoRegistra: Registrar = () => {};

/** Quem mostra "Salvo às" (o cabeçalho de detalhe) ouve todo autosave bem-sucedido da tela. */
export const RegistroDeSalvamento = createContext<Registrar>(naoRegistra);

export function useRegistrarSalvamento(): Registrar {
  return useContext(RegistroDeSalvamento);
}
