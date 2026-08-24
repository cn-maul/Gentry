import axios, { AxiosError, type AxiosResponse } from 'axios'

// API 基础 URL - 支持环境变量配置
const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || '/api/v1'

export interface ApiEnvelope<T = unknown> {
  code: number
  message: string
  data: T
}

/** 业务码异常（HTTP 成功但 code !== 0） */
export class ApiBusinessError extends Error {
  code: number
  isBusinessError = true as const
  response?: AxiosResponse

  constructor(message: string, code: number, response?: AxiosResponse) {
    super(message)
    this.code = code
    this.response = response
  }
}

function applyAuth(config: AxiosResponse['config']) {
  const token = localStorage.getItem('gentry_auth_token') || ''
  if (token) {
    config.headers = config.headers || {}
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
}

// 响应拦截器：
// 1. HTTP 成功但业务码非 0 时抛出异常（error.isBusinessError = true, error.code = 业务码），
//    页面无需再逐处判断 res.code === 0（保留判断也兼容：能走到 .then 的响应 code 必为 0）。
// 2. HTTP 错误时，如果后端带了 message，重写 e.message，页面可直接使用 e.message。
function normalizeResponse(r: AxiosResponse) {
  const body = r.data
  if (body && typeof body.code === 'number' && body.code !== 0) {
    return Promise.reject(new ApiBusinessError(body.message || '操作失败', body.code, r))
  }
  return r
}

function normalizeError(e: unknown) {
  if (e instanceof AxiosError) {
    const msg = (e.response?.data as { message?: string } | undefined)?.message
    if (msg) e.message = msg
  }
  return Promise.reject(e)
}

// 创建统一的 API 客户端
const client = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
})
client.interceptors.request.use(applyAuth)
client.interceptors.response.use(normalizeResponse, normalizeError)

// 公开接口客户端（不需要认证）
const publicClient = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10000,
})
publicClient.interceptors.response.use(normalizeResponse, normalizeError)

export { client, publicClient }

export function setAuthToken(token: string) {
  if (token) localStorage.setItem('gentry_auth_token', token)
  else localStorage.removeItem('gentry_auth_token')
}

export function getAuthToken() {
  return localStorage.getItem('gentry_auth_token') || ''
}
