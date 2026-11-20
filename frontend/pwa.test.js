import { readFileSync } from 'node:fs'
import { expect, it } from 'vitest'

it('provides a standalone launcher and correctly sized mobile icons', () => {
  const manifest = JSON.parse(readFileSync('public/manifest.webmanifest', 'utf8'))
  expect(manifest).toMatchObject({ display: 'standalone', start_url: '/', scope: '/' })
  for (const icon of [...manifest.icons, { src: '/cnc-180.png', sizes: '180x180' }]) {
    const png = readFileSync('public' + icon.src)
    expect([png.readUInt32BE(16), png.readUInt32BE(20)].join('x')).toBe(icon.sizes)
  }
})
