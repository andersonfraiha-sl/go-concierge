import type { SvgIconComponent } from '@mui/icons-material';
import ApartmentOutlined from '@mui/icons-material/ApartmentOutlined';
import EditNoteOutlined from '@mui/icons-material/EditNoteOutlined';
import EventAvailableOutlined from '@mui/icons-material/EventAvailableOutlined';
import ExtensionOutlined from '@mui/icons-material/ExtensionOutlined';
import HandymanOutlined from '@mui/icons-material/HandymanOutlined';
import HowToRegOutlined from '@mui/icons-material/HowToRegOutlined';
import Inventory2Outlined from '@mui/icons-material/Inventory2Outlined';
import KeyOutlined from '@mui/icons-material/KeyOutlined';
import LocalShippingOutlined from '@mui/icons-material/LocalShippingOutlined';

const ICONE_POR_NOME: Record<string, SvgIconComponent> = {
  truck: LocalShippingOutlined,
  'file-pen-line': EditNoteOutlined,
  'key-round': KeyOutlined,
  hammer: HandymanOutlined,
  package: Inventory2Outlined,
  'calendar-check': EventAvailableOutlined,
  'building-2': ApartmentOutlined,
  'user-check': HowToRegOutlined,
};

/** Ícone lucide (nome do Go) traduzido para o MUI; nome desconhecido cai no genérico. */
export function iconeDaSkill(icone: string): SvgIconComponent {
  return ICONE_POR_NOME[icone] ?? ExtensionOutlined;
}
