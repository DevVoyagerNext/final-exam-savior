import axios from 'axios'
import type { AxiosError, InternalAxiosRequestConfig } from 'axios'

import {
  API_BASE_URL,
  STORAGE_REFRESH_TOKEN_KEY,
  STORAGE_TOKEN_KEY,
  STORAGE_USER_KEY,
} from './config.ts'
import type {
  ApiEnvelope,
  AuthResult,
  BatchGenerateInviteCodesRequest,
  BatchGenerateInviteCodesResponse,
  ChangePasswordRequest,
  CreateInviteCodeRequest,
  CreateCategoryRequest,
  DeleteFileRequest,
  DisableUserRequest,
  FileCategory,
  FileDetail,
  FileFilters,
  FileListItem,
  GeetestValidateResult,
  InviteCodeFilters,
  InviteCodeRecord,
  LoginRequest,
  MarkNotificationReadBatchRequest,
  NotificationFilters,
  NotificationRecord,
  PagedResult,
  PreviewInfo,
  RefreshTokenRequest,
  RegisterRequest,
  RetryTaskItemParams,
  ResetPasswordRequest,
  SendEmailCodeRequest,
  SendRegisterCodeResponse,
  SignedUrlResponse,
  TaskFilters,
  TaskRecord,
  TaskItemType,
  UnreadCountResponse,
  UploadFileResponse,
  UpdateInviteRemarkRequest,
  UpdateCategoryRequest,
  UserFilters,
  UserProfile,
} from './types.ts'

const http = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10_000,
})

const refreshHttp = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10_000,
})

let refreshPromise: Promise<string | null> | null = null

export class ApiError extends Error {
  status?: number
  code?: number
  requestId?: string
  data?: unknown

  constructor(message: string, options?: { status?: number; code?: number; requestId?: string; data?: unknown }) {
    super(message)
    this.name = 'ApiError'
    this.status = options?.status
    this.code = options?.code
    this.requestId = options?.requestId
    this.data = options?.data
  }
}

function clearStoredAuth() {
  localStorage.removeItem(STORAGE_TOKEN_KEY)
  localStorage.removeItem(STORAGE_REFRESH_TOKEN_KEY)
  localStorage.removeItem(STORAGE_USER_KEY)
}

function persistAuthResult(result: AuthResult) {
  localStorage.setItem(STORAGE_TOKEN_KEY, result.accessToken)
  localStorage.setItem(STORAGE_REFRESH_TOKEN_KEY, result.refreshToken)
  localStorage.setItem(STORAGE_USER_KEY, JSON.stringify(result.user))
}

http.interceptors.request.use((config) => {
  const token = localStorage.getItem(STORAGE_TOKEN_KEY)
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

function getEnvelopeData(value: unknown): ApiEnvelope<unknown> | null {
  if (!value || typeof value !== 'object') {
    return null
  }
  const envelope = value as Partial<ApiEnvelope<unknown>>
  if (typeof envelope.code !== 'number' || typeof envelope.message !== 'string' || !('data' in envelope)) {
    return null
  }
  return envelope as ApiEnvelope<unknown>
}

http.interceptors.response.use(
  (response) => {
    const envelope = getEnvelopeData(response.data)
    if (envelope && envelope.code !== 0) {
      throw new ApiError(envelope.message || '请求失败', {
        status: response.status,
        code: envelope.code,
        requestId: typeof envelope.requestId === 'string' ? envelope.requestId : undefined,
        data: envelope.data,
      })
    }
    return response
  },
  async (error: AxiosError) => {
    const original = error.config as (InternalAxiosRequestConfig & { _retry?: boolean }) | undefined
    const isRefreshRequest = original?.url?.includes('/auth/refresh')
    const responseEnvelope = getEnvelopeData(error.response?.data)
    const businessCode = responseEnvelope?.code

    if (error.response?.status !== 401 || businessCode !== 40101 || !original || original._retry || isRefreshRequest) {
      if (error.response?.status === 401 && isRefreshRequest) {
        clearStoredAuth()
        window.location.href = '/login'
      }
      if (responseEnvelope) {
        return Promise.reject(
          new ApiError(responseEnvelope.message || error.message || '请求失败', {
            status: error.response?.status,
            code: responseEnvelope.code,
            requestId: typeof responseEnvelope.requestId === 'string' ? responseEnvelope.requestId : undefined,
            data: responseEnvelope.data,
          }),
        )
      }
      return Promise.reject(error)
    }

    const refreshToken = localStorage.getItem(STORAGE_REFRESH_TOKEN_KEY)
    if (!refreshToken) {
      clearStoredAuth()
      window.location.href = '/login'
      return Promise.reject(error)
    }

    original._retry = true
    try {
      if (!refreshPromise) {
        refreshPromise = authApi.refresh(refreshToken)
          .then((result) => {
            persistAuthResult(result)
            return result.accessToken
          })
          .finally(() => {
            refreshPromise = null
          })
      }
      const accessToken = await refreshPromise
      if (!accessToken) {
        throw new Error('refresh access token failed')
      }
      original.headers = original.headers ?? {}
      original.headers.Authorization = `Bearer ${accessToken}`
      return http(original)
    } catch (refreshError) {
      clearStoredAuth()
      window.location.href = '/login'
      return Promise.reject(refreshError)
    }
  }
)

async function unwrap<T>(promise: Promise<{ data: ApiEnvelope<T> }>) {
  const response = await promise
  return response.data.data
}

function isProtectedPreviewUrl(resourceUrl: string) {
  if (!resourceUrl) {
    return false
  }
  if (resourceUrl.startsWith('/')) {
    return resourceUrl.startsWith('/api/')
  }
  try {
    const parsed = new URL(resourceUrl, window.location.origin)
    return parsed.origin === window.location.origin && parsed.pathname.startsWith('/api/')
  } catch {
    return false
  }
}

function normalizeProtectedRequestUrl(resourceUrl: string) {
  const apiBase = API_BASE_URL.trim()
  if (!apiBase) {
    return resourceUrl
  }

  if (resourceUrl.startsWith('http://') || resourceUrl.startsWith('https://')) {
    const parsed = new URL(resourceUrl)
    if (parsed.origin !== window.location.origin) {
      return resourceUrl
    }
    const pathWithQuery = `${parsed.pathname}${parsed.search}`
    if (apiBase.startsWith('/') && pathWithQuery.startsWith(apiBase)) {
      const stripped = pathWithQuery.slice(apiBase.length)
      return stripped || '/'
    }
    return pathWithQuery
  }

  if (apiBase.startsWith('/') && resourceUrl.startsWith(apiBase)) {
    const stripped = resourceUrl.slice(apiBase.length)
    return stripped || '/'
  }

  return resourceUrl
}

export async function resolveOpenableUrl(resourceUrl: string) {
  if (!isProtectedPreviewUrl(resourceUrl)) {
    return resourceUrl
  }
  const response = await http.get<Blob>(normalizeProtectedRequestUrl(resourceUrl), { responseType: 'blob' })
  const blobUrl = window.URL.createObjectURL(response.data)
  window.setTimeout(() => window.URL.revokeObjectURL(blobUrl), 5 * 60 * 1000)
  return blobUrl
}

function withCaptchaPayload<T extends object>(payload: T, captchaData: GeetestValidateResult) {
  return {
    ...payload,
    lot_number: captchaData.lot_number,
    captcha_output: captchaData.captcha_output,
    pass_token: captchaData.pass_token,
    gen_time: captchaData.gen_time,
    captcha_id: captchaData.captcha_id,
    sign_token: captchaData.sign_token,
  }
}

export const authApi = {
  async login(payload: LoginRequest) {
    return unwrap<AuthResult>(
      http.post(
        '/auth/login',
        withCaptchaPayload(
          {
            email: payload.email,
            password: payload.password,
          },
          payload.captchaData,
        ),
      ),
    )
  },

  async register(payload: RegisterRequest) {
    return unwrap<AuthResult>(
      http.post('/auth/register', {
        email: payload.email,
        emailCode: payload.emailCode,
        password: payload.password,
        confirmPassword: payload.confirmPassword,
        inviteCode: payload.inviteCode,
      }),
    )
  },

  async sendRegisterCode(payload: SendEmailCodeRequest) {
    return unwrap<SendRegisterCodeResponse>(
      http.post(
        '/auth/register/email-code/send',
        withCaptchaPayload(
          {
            email: payload.email,
          },
          payload.captchaData,
        ),
      ),
    )
  },

  async sendResetCode(payload: SendEmailCodeRequest) {
    return unwrap<void>(
      http.post(
        '/auth/password-reset/email-code/send',
        withCaptchaPayload(
          {
            email: payload.email,
          },
          payload.captchaData,
        ),
      ),
    )
  },

  async resetPassword(payload: ResetPasswordRequest) {
    return unwrap<void>(http.post('/auth/password-reset/confirm', payload))
  },

  async refresh(refreshToken: string) {
    const payload: RefreshTokenRequest = { refreshToken }
    return unwrap<AuthResult>(refreshHttp.post('/auth/refresh', payload))
  },

  async me() {
    return unwrap<UserProfile>(http.get('/auth/me'))
  },

  async changePassword(payload: ChangePasswordRequest) {
    return unwrap<void>(http.post('/auth/password/change', payload))
  },

  async logout() {
    return unwrap<void>(http.post('/auth/logout'))
  },
}

export const fileApi = {
  async listFiles(filters: FileFilters) {
    return unwrap<PagedResult<FileListItem>>(http.get('/files', { params: filters }))
  },

  async getFileDetail(fileId: number) {
    return unwrap<FileDetail>(http.get(`/files/${fileId}`))
  },

  async getCategories() {
    return unwrap<FileCategory[]>(http.get('/file-categories'))
  },

  async upload(formData: FormData) {
    return unwrap<UploadFileResponse>(
      http.post('/admin/files/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } }),
    )
  },

  async deleteFile(fileId: number, confirmText: string) {
    const payload: DeleteFileRequest = { confirmText }
    return unwrap<void>(http.delete(`/admin/files/${fileId}`, { data: payload }))
  },

  async previewSource(fileId: number) {
    return unwrap<PreviewInfo>(http.get(`/files/${fileId}/preview-source`))
  },

  async previewResult(fileId: number, itemType: TaskItemType) {
    return unwrap<PreviewInfo>(http.get(`/files/${fileId}/preview-result`, { params: { itemType } }))
  },

  async previewResultHtml(fileId: number, itemType: TaskItemType) {
    const response = await http.get<string>(`/files/${fileId}/view-result`, {
      params: { itemType },
      responseType: 'text',
    })
    return response.data
  },

  async downloadSource(fileId: number) {
    return unwrap<SignedUrlResponse>(http.get(`/files/${fileId}/download-source`))
  },

  async downloadResult(fileId: number, itemType: TaskItemType) {
    return unwrap<SignedUrlResponse>(http.get(`/files/${fileId}/download-result`, { params: { itemType } }))
  },
}

export const taskApi = {
  async listTasks(filters: TaskFilters) {
    return unwrap<PagedResult<TaskRecord>>(http.get('/tasks', { params: filters }))
  },

  async getTask(taskId: number) {
    return unwrap<TaskRecord>(http.get(`/tasks/${taskId}`))
  },

  async retryTaskItem(params: RetryTaskItemParams) {
    return unwrap<void>(http.post(`/admin/tasks/${params.taskId}/items/${params.taskItemId}/retry`))
  },
}

export const notificationApi = {
  async listNotifications(filters: NotificationFilters) {
    return unwrap<PagedResult<NotificationRecord>>(http.get('/notifications', { params: filters }))
  },

  async getNotification(notificationId: number) {
    return unwrap<NotificationRecord>(http.get(`/notifications/${notificationId}`))
  },

  async markRead(notificationId: number) {
    return unwrap<void>(http.post(`/notifications/${notificationId}/read`))
  },

  async markBatchRead(notificationIds: number[]) {
    const payload: MarkNotificationReadBatchRequest = { notificationIds }
    return unwrap<void>(http.post('/notifications/read/batch', payload))
  },

  async unreadCount() {
    return unwrap<UnreadCountResponse>(http.get('/notifications/unread-count'))
  },
}

export const adminApi = {
  async listUsers(filters: UserFilters) {
    return unwrap<PagedResult<UserProfile>>(http.get('/admin/users', { params: filters }))
  },

  async enableUser(userId: number) {
    return unwrap<void>(http.post(`/admin/users/${userId}/enable`))
  },

  async disableUser(userId: number, remark: string) {
    const payload: DisableUserRequest = { remark }
    return unwrap<void>(http.post(`/admin/users/${userId}/disable`, payload))
  },

  async listAdminFiles(filters: FileFilters) {
    return unwrap<PagedResult<FileListItem>>(http.get('/admin/files', { params: filters }))
  },

  async listInviteCodes(filters: InviteCodeFilters) {
    return unwrap<PagedResult<InviteCodeRecord>>(http.get('/admin/invite-codes', { params: filters }))
  },

  async createInviteCode(payload: CreateInviteCodeRequest) {
    return unwrap<void>(http.post('/admin/invite-codes', payload))
  },

  async batchGenerateInviteCodes(payload: BatchGenerateInviteCodesRequest) {
    return unwrap<BatchGenerateInviteCodesResponse>(
      http.post('/admin/invite-codes/batch-generate', payload),
    )
  },

  async updateInviteRemark(inviteCodeId: number, remark: string) {
    const payload: UpdateInviteRemarkRequest = { remark }
    return unwrap<void>(http.put(`/admin/invite-codes/${inviteCodeId}/remark`, payload))
  },

  async deleteInviteCode(inviteCodeId: number) {
    return unwrap<void>(http.delete(`/admin/invite-codes/${inviteCodeId}`))
  },

  async createCategory(payload: CreateCategoryRequest) {
    return unwrap<void>(http.post('/admin/file-categories', payload))
  },

  async updateCategory(categoryId: number, payload: UpdateCategoryRequest) {
    return unwrap<void>(http.put(`/admin/file-categories/${categoryId}`, payload))
  },

  async deleteCategory(categoryId: number) {
    return unwrap<void>(http.delete(`/admin/file-categories/${categoryId}`))
  },
}
