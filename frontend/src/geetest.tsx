import { Typography } from 'antd'
import { CheckCircleFilled, LoadingOutlined } from '@ant-design/icons'
import { useEffect, useId, useMemo, useState } from 'react'

import { GEETEST_CAPTCHA_ID, GEETEST_SCRIPT_URL } from './config.ts'
import type { GeetestValidateResult } from './types.ts'

interface GeetestInstance {
  appendTo: (selector: string) => void
  getValidate: () =>
    | {
        lot_number: string
        captcha_output: string
        pass_token: string
        gen_time: string
        sign_token: string
      }
    | undefined
  reset?: () => void
  destroy?: () => void
  onReady?: (callback: () => void) => void
  onSuccess?: (callback: () => void) => void
  onError?: (callback: () => void) => void
}

declare global {
  interface Window {
    initGeetest4?: (
      options: {
        captchaId: string
        protocol?: 'http://' | 'https://' | 'file://'
      },
      callback: (captcha: GeetestInstance) => void,
    ) => void
  }
}

function ensureGeetestScript() {
  return new Promise<void>((resolve, reject) => {
    if (window.initGeetest4) {
      resolve()
      return
    }

    const existing = document.querySelector<HTMLScriptElement>('script[data-geetest-script="gt4"]')
    if (existing) {
      existing.addEventListener('load', () => resolve(), { once: true })
      existing.addEventListener('error', () => reject(new Error('极验脚本加载失败')), {
        once: true,
      })
      return
    }

    const script = document.createElement('script')
    script.src = GEETEST_SCRIPT_URL
    script.async = true
    script.dataset.geetestScript = 'gt4'
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('极验脚本加载失败'))
    document.head.appendChild(script)
  })
}

export function GeetestCaptchaPanel(props: {
  sceneLabel: string
  value: GeetestValidateResult | null
  onChange: (value: GeetestValidateResult | null) => void
}) {
  const { onChange, value } = props
  const rawId = useId()
  const containerId = useMemo(() => `gt4-${rawId.replace(/[:]/g, '')}`, [rawId])
  const [status, setStatus] = useState<'initializing' | 'ready' | 'success' | 'error'>(
    GEETEST_CAPTCHA_ID ? 'initializing' : 'error',
  )
  const [errorText, setErrorText] = useState(GEETEST_CAPTCHA_ID ? '' : '未配置 VITE_GEETEST_CAPTCHA_ID，无法初始化极验 GT4。')

  useEffect(() => {
    onChange(null)

    if (!GEETEST_CAPTCHA_ID) {
      return
    }

    let destroyed = false
    let captchaRef: GeetestInstance | null = null

    void ensureGeetestScript()
      .then(() => {
        if (destroyed || !window.initGeetest4) {
          return
        }

        window.initGeetest4(
          {
            captchaId: GEETEST_CAPTCHA_ID,
            protocol: 'https://',
          },
          (captcha) => {
            if (destroyed) {
              captcha.destroy?.()
              return
            }

            captchaRef = captcha
            captcha.appendTo(`#${containerId}`)
            captcha.onReady?.(() => {
              if (!destroyed) {
                setStatus('ready')
              }
            })
            captcha.onSuccess?.(() => {
              const validate = captcha.getValidate()
              if (!validate) {
                return
              }
              onChange({
                ...validate,
                captcha_id: GEETEST_CAPTCHA_ID,
              })
              setStatus('success')
            })
            captcha.onError?.(() => {
              if (!destroyed) {
                setStatus('error')
                setErrorText('极验校验执行失败，请刷新页面后重试。')
              }
            })
          },
        )
      })
      .catch((error: Error) => {
        if (!destroyed) {
          setStatus('error')
          setErrorText(error.message)
        }
      })

    return () => {
      destroyed = true
      onChange(null)
      captchaRef?.destroy?.()
    }
  }, [containerId, onChange])

  return (
    <div className={`security-check security-check--${status}`}>
      <div className="security-check__header">
        <Typography.Text strong>安全验证</Typography.Text>
        <div className="security-check__state">
          {status === 'initializing' ? (
            <>
              <LoadingOutlined />
              <span>加载中</span>
            </>
          ) : null}
          {status === 'success' || value ? (
            <>
              <CheckCircleFilled />
              <span>已完成</span>
            </>
          ) : null}
          {status === 'error' ? <span>暂时不可用</span> : null}
          {status === 'ready' ? <span>请完成验证</span> : null}
        </div>
      </div>

      <div id={containerId} className="geetest-container" />

      {status === 'error' ? (
        <Typography.Text type="danger">{errorText || '安全验证加载失败，请刷新页面后重试。'}</Typography.Text>
      ) : null}
    </div>
  )
}
