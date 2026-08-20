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
import i18next from 'i18next'

import type { ApiResponse } from '@/features/auth/types'
import { api, get2FAStatus } from '@/lib/api'

import type {
  SecurityProof,
  SecurityProofScope,
  VerificationMethod,
  VerificationMethods,
} from './types'

/**
 * Fetch available verification methods for the current user.
 */
export async function checkVerificationMethods(): Promise<VerificationMethods> {
  try {
    const twoFAResponse = await get2FAStatus()
    const has2FA =
      Boolean(twoFAResponse?.success) && Boolean(twoFAResponse?.data?.enabled)

    return {
      has2FA,
      hasPasskey: false,
      passkeySupported: false,
    }
  } catch (error) {
    // eslint-disable-next-line no-console
    console.error('[Secure Verification] Failed to check methods', error)
    return {
      has2FA: false,
      hasPasskey: false,
      passkeySupported: false,
    }
  }
}

/**
 * Execute a 2FA verification flow.
 */
export async function verify(
  method: VerificationMethod,
  scope: SecurityProofScope,
  code?: string
): Promise<SecurityProof> {
  if (method !== '2fa') {
    throw new Error(
      i18next.t('Unsupported verification method: {{method}}', { method })
    )
  }

  const trimmed = code?.trim()
  if (!trimmed) {
    throw new Error(
      i18next.t('Please enter the verification code or backup code')
    )
  }

  const res = await api.post<ApiResponse<SecurityProof>>('/api/verify', {
    method: '2fa',
    code: trimmed,
    scope,
  })

  if (!res.data?.success) {
    throw new Error(res.data?.message || i18next.t('Verification failed'))
  }
  if (!res.data.data?.proof_token) {
    throw new Error(i18next.t('Verification proof was not returned'))
  }
  return res.data.data
}