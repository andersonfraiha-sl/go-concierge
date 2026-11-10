const SO_DATA = /^(\d{4})-(\d{2})-(\d{2})$/;

/**
 * Data em dia/mês/ano. "AAAA-MM-DD" é lida como está, sem passar por fuso (senão o dia anda);
 * data e hora vira o dia local de quem olha.
 */
export function formatarData(valor: string): string {
  const soData = SO_DATA.exec(valor);
  if (soData) return `${soData[3]}/${soData[2]}/${soData[1]}`;
  const data = new Date(valor);
  return Number.isNaN(data.getTime()) ? valor : data.toLocaleDateString('pt-BR');
}
