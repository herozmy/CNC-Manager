export type DrillingCycle = 'G81' | 'G83' | 'G84'
export const RIGID_TAPPING_MANUAL = 'https://www.emco-world.com/fileadmin/pim/EMCO_WinNC_for_Fanuc31i_Mill_EN_1846_C_01.pdf#page=243'

// Integer thousandths keep the G94 feed and entered pitch consistent at NC precision.
export function rigidTappingFeed(pitchText: string, rpmText: string): number | null {
  const pitch = /^\+?(?:\d+(?:\.\d{0,3})?|\.\d{1,3})$/.test(pitchText.trim()) ? Number(pitchText) : NaN
  const rpm = /^\d+$/.test(rpmText.trim()) ? Number(rpmText) : NaN
  if (!Number.isFinite(pitch) || pitch < .001 || pitch > 20 || !Number.isSafeInteger(rpm) || rpm < 1 || rpm > 99999) return null
  const milliFeed = Math.round(pitch * 1000) * rpm
  return milliFeed <= 99999999 ? milliFeed / 1000 : null
}
