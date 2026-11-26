import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router-dom'
import { afterEach, expect, test, vi } from 'vitest'
import { detail, mockApi, operation, program, renderApp } from '../test-utils'
import { DrawingPreparationPage } from './PreparationPage'

afterEach(() => vi.unstubAllGlobals())

test('切换来源时分别带入台账和刀具，保留手填补充项', async () => {
  mockApi(({ path }) => {
    if (path === '/drawings/1/detail') return { body: detail([
      operation({ programs: [program({ currentVersionId: 8 })] }),
      operation({ id: 2, opNo: 20, programs: [] }),
    ]) }
    if (path === '/versions/8/tools') return { body: { items: [{ toolNo: 'T08', compAmount: 0 }] } }
  })
  renderApp(<Routes><Route path="/drawings/:id/preparation" element={<DrawingPreparationPage />} /></Routes>, '/drawings/1/preparation')
  const u = userEvent.setup()
  expect(await screen.findByDisplayValue('D-1001')).toBeVisible()
  expect(screen.getByLabelText('刀号')).toHaveValue('T08')
  expect(screen.getByLabelText('刀补（mm）')).toHaveValue('0')
  await u.type(screen.getByLabelText('补充事项'), 'inspect')
  await u.selectOptions(screen.getByLabelText('来源工序'), '2')
  expect(screen.getByLabelText('补充事项')).toHaveValue('')
  expect(screen.getByLabelText('刀号')).toHaveValue('')
  await u.selectOptions(screen.getByLabelText('来源工序'), '1')
  expect(screen.getByLabelText('补充事项')).toHaveValue('inspect')
})
