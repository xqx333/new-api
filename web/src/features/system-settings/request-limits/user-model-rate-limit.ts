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
export const MAX_RATE_LIMIT_VALUE = 2147483647

export type UserModelRateLimitRule = {
  user_id: number
  model: string
  max_requests: number
  max_success: number
}

function isValidRule(value: unknown): value is UserModelRateLimitRule {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    return false
  }

  const rule = value as Record<string, unknown>
  return (
    typeof rule.user_id === 'number' &&
    Number.isInteger(rule.user_id) &&
    rule.user_id > 0 &&
    typeof rule.model === 'string' &&
    rule.model.length > 0 &&
    rule.model.trim() === rule.model &&
    typeof rule.max_requests === 'number' &&
    Number.isInteger(rule.max_requests) &&
    rule.max_requests >= 0 &&
    rule.max_requests <= MAX_RATE_LIMIT_VALUE &&
    typeof rule.max_success === 'number' &&
    Number.isInteger(rule.max_success) &&
    rule.max_success >= 0 &&
    rule.max_success <= MAX_RATE_LIMIT_VALUE &&
    (rule.max_requests > 0 || rule.max_success > 0)
  )
}

function hasDuplicateRules(rules: UserModelRateLimitRule[]): boolean {
  const identities = new Set<string>()
  for (const rule of rules) {
    const identity = `${rule.user_id}\u0000${rule.model}`
    if (identities.has(identity)) return true
    identities.add(identity)
  }
  return false
}

export function parseUserModelRateLimitRules(
  value: string
): UserModelRateLimitRule[] {
  if (!value.trim()) return []

  try {
    const parsed: unknown = JSON.parse(value)
    if (!Array.isArray(parsed) || !parsed.every(isValidRule)) return []
    return parsed
  } catch {
    return []
  }
}

export function isValidUserModelRateLimitJSON(value: string): boolean {
  if (!value.trim()) return false

  try {
    const parsed: unknown = JSON.parse(value)
    return (
      Array.isArray(parsed) &&
      parsed.every(isValidRule) &&
      !hasDuplicateRules(parsed)
    )
  } catch {
    return false
  }
}

export function serializeUserModelRateLimitRules(
  rules: UserModelRateLimitRule[]
): string {
  return JSON.stringify(rules, null, 2)
}

export function hasUserModelRateLimitRule(
  rules: UserModelRateLimitRule[],
  userId: number,
  model: string,
  ignoredRule?: UserModelRateLimitRule | null
): boolean {
  let ignored = false
  for (const rule of rules) {
    if (
      !ignored &&
      ignoredRule &&
      rule.user_id === ignoredRule.user_id &&
      rule.model === ignoredRule.model
    ) {
      ignored = true
      continue
    }
    if (rule.user_id === userId && rule.model === model) return true
  }
  return false
}

export function upsertUserModelRateLimitRule(
  rules: UserModelRateLimitRule[],
  rule: UserModelRateLimitRule,
  previousRule?: UserModelRateLimitRule | null
): UserModelRateLimitRule[] {
  if (!previousRule) return [...rules, rule]

  const index = rules.findIndex(
    (item) =>
      item.user_id === previousRule.user_id && item.model === previousRule.model
  )
  if (index < 0) return [...rules, rule]

  const updated = [...rules]
  updated[index] = rule
  return updated
}

export function removeUserModelRateLimitRule(
  rules: UserModelRateLimitRule[],
  userId: number,
  model: string
): UserModelRateLimitRule[] {
  return rules.filter((rule) => rule.user_id !== userId || rule.model !== model)
}
