/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { RateLimitSection } from '../rate-limit-section'

const updateOptionMock = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({
    isPending: false,
    mutateAsync: updateOptionMock.mutateAsync,
  }),
}))

vi.mock('react-i18next', async () => {
  const actual =
    await vi.importActual<typeof import('react-i18next')>('react-i18next')
  return {
    ...actual,
    useTranslation: () => ({
      t: (key: string) => key,
    }),
  }
})

function renderRateLimitSection(hideDetails: boolean) {
  const actionsContainer = document.createElement('div')
  document.body.append(actionsContainer)

  render(
    <SettingsPageProvider actionsContainer={actionsContainer}>
      <RateLimitSection
        defaultValues={{
          ModelRequestRateLimitEnabled: true,
          ModelRequestRateLimitHideDetailsEnabled: hideDetails,
          ModelRequestRateLimitDurationMinutes: 1,
          ModelRequestRateLimitCount: 100,
          ModelRequestRateLimitSuccessCount: 0,
          ModelRequestRateLimitGroup: '{}',
          ModelRequestRateLimitUserModel: '[]',
        }}
      />
    </SettingsPageProvider>
  )
}

beforeEach(() => {
  updateOptionMock.mutateAsync.mockReset()
  updateOptionMock.mutateAsync.mockResolvedValue({ success: true })
})

afterEach(() => {
  cleanup()
  document.body.innerHTML = ''
})

describe('rate limit response detail setting', () => {
  test('submits true after the administrator enables the switch', async () => {
    const user = userEvent.setup()
    renderRateLimitSection(false)

    const hideDetailsSwitch = screen.getByRole('switch', {
      name: 'Hide rate limit details',
    })
    expect(hideDetailsSwitch.getAttribute('aria-checked')).toBe('false')

    await user.click(hideDetailsSwitch)
    expect(hideDetailsSwitch.getAttribute('aria-checked')).toBe('true')
    await user.click(screen.getByRole('button', { name: 'Save rate limits' }))

    await waitFor(() => {
      expect(updateOptionMock.mutateAsync).toHaveBeenCalledTimes(1)
      expect(updateOptionMock.mutateAsync).toHaveBeenCalledWith({
        key: 'ModelRequestRateLimitHideDetailsEnabled',
        value: true,
      })
    })
  })

  test('submits false after the administrator disables the switch', async () => {
    const user = userEvent.setup()
    renderRateLimitSection(true)

    const hideDetailsSwitch = screen.getByRole('switch', {
      name: 'Hide rate limit details',
    })
    expect(hideDetailsSwitch.getAttribute('aria-checked')).toBe('true')

    await user.click(hideDetailsSwitch)
    expect(hideDetailsSwitch.getAttribute('aria-checked')).toBe('false')
    await user.click(screen.getByRole('button', { name: 'Save rate limits' }))

    await waitFor(() => {
      expect(updateOptionMock.mutateAsync).toHaveBeenCalledTimes(1)
      expect(updateOptionMock.mutateAsync).toHaveBeenCalledWith({
        key: 'ModelRequestRateLimitHideDetailsEnabled',
        value: false,
      })
    })
  })
})
