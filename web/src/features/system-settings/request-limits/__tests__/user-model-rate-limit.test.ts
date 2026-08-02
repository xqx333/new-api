/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  hasUserModelRateLimitRule,
  isValidUserModelRateLimitJSON,
  parseUserModelRateLimitRules,
  removeUserModelRateLimitRule,
  serializeUserModelRateLimitRules,
  upsertUserModelRateLimitRule,
  type UserModelRateLimitRule,
} from '../user-model-rate-limit.ts'

const initialRules: UserModelRateLimitRule[] = [
  {
    user_id: 123,
    model: 'gpt-5.4',
    max_requests: 20,
    max_success: 0,
  },
]

describe('user-model rate limit configuration', () => {
  test('round-trips valid visual editor rules through JSON', () => {
    const serialized = serializeUserModelRateLimitRules(initialRules)

    assert.deepEqual(parseUserModelRateLimitRules(serialized), initialRules)
    assert.equal(isValidUserModelRateLimitJSON(serialized), true)
  })

  test('accepts either zero limit but rejects rules where both are zero', () => {
    assert.equal(
      isValidUserModelRateLimitJSON(
        '[{"user_id":123,"model":"gpt-5.4","max_requests":0,"max_success":10}]'
      ),
      true
    )
    assert.equal(
      isValidUserModelRateLimitJSON(
        '[{"user_id":123,"model":"gpt-5.4","max_requests":0,"max_success":0}]'
      ),
      false
    )
  })

  test('detects duplicate user and exact-model identities', () => {
    const duplicate = [...initialRules, { ...initialRules[0] }]

    assert.equal(
      isValidUserModelRateLimitJSON(
        serializeUserModelRateLimitRules(duplicate)
      ),
      false
    )
    assert.equal(hasUserModelRateLimitRule(initialRules, 123, 'gpt-5.4'), true)
    assert.equal(hasUserModelRateLimitRule(initialRules, 123, 'GPT-5.4'), false)
  })

  test('updates and deletes only the selected rule', () => {
    const updatedRule = {
      ...initialRules[0],
      model: 'gpt-5.4-mini',
      max_requests: 30,
    }
    const updated = upsertUserModelRateLimitRule(
      initialRules,
      updatedRule,
      initialRules[0]
    )

    assert.deepEqual(updated, [updatedRule])
    assert.deepEqual(
      removeUserModelRateLimitRule(updated, 123, 'gpt-5.4-mini'),
      []
    )
  })
})
