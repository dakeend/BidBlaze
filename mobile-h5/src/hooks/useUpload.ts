import { useCallback, useEffect, useRef, useState } from 'react'
import { uploadImage } from '../lib/api-client'
import type { UploadResult } from '../lib/types'

const maxUploadSize = 5 * 1024 * 1024
const allowedTypes = new Set(['image/jpeg', 'image/png', 'image/webp'])

export type UploadStatus = 'idle' | 'validating' | 'uploading' | 'success' | 'error'

type UseUploadState = {
  status: UploadStatus
  progress: number
  error: string | null
  result: UploadResult | null
}

function validateImage(file: File) {
  if (!allowedTypes.has(file.type)) {
    throw new Error('仅支持 JPG、PNG、WebP 图片')
  }
  if (file.size > maxUploadSize) {
    throw new Error('图片大小不能超过 5MB')
  }
}

export function useUpload() {
  const mountedRef = useRef(true)
  const [state, setState] = useState<UseUploadState>({
    status: 'idle',
    progress: 0,
    error: null,
    result: null,
  })

  useEffect(() => {
    return () => {
      mountedRef.current = false
    }
  }, [])

  const safeSetState = useCallback((next: Partial<UseUploadState>) => {
    if (!mountedRef.current) {
      return
    }
    setState((current) => ({ ...current, ...next }))
  }, [])

  const reset = useCallback(() => {
    safeSetState({
      status: 'idle',
      progress: 0,
      error: null,
      result: null,
    })
  }, [safeSetState])

  const upload = useCallback(
    async (file: File) => {
      safeSetState({ status: 'validating', progress: 0, error: null, result: null })
      try {
        validateImage(file)
      } catch (error) {
        const message = error instanceof Error ? error.message : '图片校验失败'
        safeSetState({ status: 'error', error: message })
        throw error
      }

      let lastError: unknown
      for (let attempt = 0; attempt < 2; attempt += 1) {
        try {
          safeSetState({ status: 'uploading', error: null })
          const result = await uploadImage(file, (progress) => {
            safeSetState({ progress })
          })
          safeSetState({ status: 'success', progress: 100, result, error: null })
          return result
        } catch (error) {
          lastError = error
          if (attempt === 0) {
            safeSetState({ progress: 0 })
          }
        }
      }

      const message = lastError instanceof Error ? lastError.message : '上传失败，请稍后重试'
      safeSetState({ status: 'error', error: message })
      throw lastError instanceof Error ? lastError : new Error(message)
    },
    [safeSetState],
  )

  return {
    ...state,
    upload,
    reset,
    maxUploadSize,
    allowedTypes,
  }
}
