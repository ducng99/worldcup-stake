const TLA_MAP: Record<string, string> = {
  // UEFA
  FRA: 'fr', GER: 'de', ESP: 'es', POR: 'pt',
  ENG: 'gb-eng', NED: 'nl', BEL: 'be', CRO: 'hr',
  SUI: 'ch', BIH: 'ba', CZE: 'cz', AUT: 'at',
  SCO: 'gb-sct', TUR: 'tr', NOR: 'no', SWE: 'se',
  // CONMEBOL
  ARG: 'ar', BRA: 'br', COL: 'co', ECU: 'ec',
  URY: 'uy', PAR: 'py',
  // CONCACAF
  USA: 'us', MEX: 'mx', CAN: 'ca', PAN: 'pa',
  HAI: 'ht', CUW: 'cw',
  // CAF
  MAR: 'ma', SEN: 'sn', EGY: 'eg', GHA: 'gh',
  ALG: 'dz', COD: 'cd', TUN: 'tn', RSA: 'za',
  CIV: 'ci',
  // AFC
  JPN: 'jp', KOR: 'kr', AUS: 'au', IRN: 'ir',
  KSA: 'sa', UZB: 'uz', JOR: 'jo', IRQ: 'iq',
  QAT: 'qa',
  // OFC + other
  CPV: 'cv', NZL: 'nz',
}

export function getFlagClass(code: string): string {
  const iso2 = TLA_MAP[code]
  return iso2 ? `fi fi-${iso2}` : ''
}
