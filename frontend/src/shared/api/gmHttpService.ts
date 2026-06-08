import axios, { AxiosInstance, AxiosResponse } from 'axios'
import Cookie from '@/shared/lib/cookie'
import { GM_API_URL } from '@/shared/config/env'

interface GmEnvelope<T> {
  status: 'success' | 'fail'
  data?: T
  message?: string
}

function unwrap<T>(response: AxiosResponse<GmEnvelope<T>>): T {
  const env = response.data
  if (env.status === 'fail') {
    const err = new Error(env.message ?? 'Request failed')
    ;(err as Error & { status: number }).status = response.status
    throw err
  }
  return env.data as T
}

class GmHttpService {
  protected request(): AxiosInstance {
    const token = Cookie.get('gm_token')
    const instance = axios.create({
      baseURL: GM_API_URL,
      withCredentials: true,
      headers: {
        'Content-Type': 'application/json',
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
    })

    // Redirect to login on 401.
    instance.interceptors.response.use(
      r => r,
      err => {
        if (err.response?.status === 401 && typeof window !== 'undefined') {
          Cookie.remove('gm_token')
          window.location.href = '/login'
        }
        return Promise.reject(err)
      },
    )

    return instance
  }

  setToken(token: string): void {
    Cookie.set('gm_token', token, { expires: 30 })
  }

  getToken(): string | undefined {
    return Cookie.get('gm_token')
  }

  clearToken(): void {
    Cookie.remove('gm_token')
  }

  get<T>(endpoint: string, params?: Record<string, unknown>): Promise<T> {
    return this.request()
      .get<GmEnvelope<T>>(endpoint, { params })
      .then(unwrap<T>)
  }

  post<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request().post<GmEnvelope<T>>(endpoint, data).then(unwrap<T>)
  }

  put<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request().put<GmEnvelope<T>>(endpoint, data).then(unwrap<T>)
  }

  delete<T>(endpoint: string): Promise<T> {
    return this.request().delete<GmEnvelope<T>>(endpoint).then(unwrap<T>)
  }
}

export default GmHttpService
