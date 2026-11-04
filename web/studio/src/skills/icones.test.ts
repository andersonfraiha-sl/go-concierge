import ApartmentOutlined from '@mui/icons-material/ApartmentOutlined';
import EditNoteOutlined from '@mui/icons-material/EditNoteOutlined';
import EventAvailableOutlined from '@mui/icons-material/EventAvailableOutlined';
import ExtensionOutlined from '@mui/icons-material/ExtensionOutlined';
import HandymanOutlined from '@mui/icons-material/HandymanOutlined';
import HowToRegOutlined from '@mui/icons-material/HowToRegOutlined';
import Inventory2Outlined from '@mui/icons-material/Inventory2Outlined';
import KeyOutlined from '@mui/icons-material/KeyOutlined';
import LocalShippingOutlined from '@mui/icons-material/LocalShippingOutlined';
import { describe, expect, it } from 'vitest';
import { iconeDaSkill } from './icones';

describe('iconeDaSkill', () => {
  it.each([
    ['truck', LocalShippingOutlined],
    ['file-pen-line', EditNoteOutlined],
    ['key-round', KeyOutlined],
    ['hammer', HandymanOutlined],
    ['package', Inventory2Outlined],
    ['calendar-check', EventAvailableOutlined],
    ['building-2', ApartmentOutlined],
    ['user-check', HowToRegOutlined],
  ])('mapeia "%s" para seu ícone MUI', (icone, esperado) => {
    expect(iconeDaSkill(icone)).toBe(esperado);
  });

  it('mapsUnknownToFallback', () => {
    expect(iconeDaSkill('nome-que-nao-existe')).toBe(ExtensionOutlined);
  });
});
