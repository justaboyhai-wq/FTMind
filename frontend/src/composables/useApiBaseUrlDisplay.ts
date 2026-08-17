import { computed, onMounted, ref } from 'vue'
import { getApiBaseUrl } from '@/utils/api-base'

type FMindDesktopWindow = Window & {
  __FMIND_API_BASE__?: string
  go?: {
    main?: {
      App?: {
        GetAPIBaseURL?: () => Promise<string> | string
      }
    }
  }
}

export function useApiBaseUrlDisplay() {
  const wailsApiBaseURL = ref<string | null>(null)

  const apiBaseUrlDisplay = computed(() => {
    if (wailsApiBaseURL.value) {
      return wailsApiBaseURL.value
    }
    const configured = getApiBaseUrl().trim().replace(/\/$/, '')
    let origin = typeof window !== 'undefined' ? window.location.origin : ''
    if (!origin || origin === 'null') {
      origin = ''
    }
    // The Vite dev server proxies /api to the backend, which works for the
    // browser but is not a usable API endpoint for an external agent. Show the
    // real local API listener in integration instructions instead.
    if (import.meta.env.DEV && window.location.port === '5173' && !configured) {
      return `${window.location.protocol}//${window.location.hostname}:8080/api/v1`
    }
    const base = configured || origin
    return `${base}/api/v1`
  })

  async function loadApiBaseUrl() {
    const win = window as FMindDesktopWindow
    for (let i = 0; i < 40; i++) {
      const injected = win.__FMIND_API_BASE__
      if (typeof injected === 'string' && injected.trim()) {
        wailsApiBaseURL.value = injected.trim().replace(/\/$/, '')
        return
      }
      const fn = win.go?.main?.App?.GetAPIBaseURL
      if (typeof fn === 'function') {
        try {
          const raw = await Promise.resolve(fn())
          if (typeof raw === 'string' && raw.trim()) {
            wailsApiBaseURL.value = raw.trim().replace(/\/$/, '')
            return
          }
        } catch {
          /* binding error */
        }
      }
      await new Promise<void>((resolve) => setTimeout(resolve, 50))
    }
  }

  onMounted(() => {
    void loadApiBaseUrl()
  })

  return { apiBaseUrlDisplay }
}
