import axios from 'axios'

import { API_BASE_URL, ENABLE_DEMO_MODE, STORAGE_TOKEN_KEY } from './config.ts'
import {
  demoAuthResult,
  demoCategories,
  demoFiles,
  demoInviteCodes,
  demoNotifications,
  demoTasks,
  demoUsers,
  getDemoFileDetail,
  getDemoNotification,
  getDemoTask,
} from './demo-data.ts'
import type {
  ApiEnvelope,
  AuthResult,
  FileCategory,
  FileDetail,
  FileFilters,
  FileListItem,
  GeetestValidateResult,
  InviteCodeFilters,
  InviteCodeRecord,
  NotificationFilters,
  NotificationRecord,
  PagedResult,
  PreviewInfo,
  TaskFilters,
  TaskRecord,
  TaskItemType,
  UserFilters,
  UserProfile,
} from './types.ts'

const http = axios.create({
  baseURL: API_BASE_URL,
  timeout: 10_000,
})

http.interceptors.request.use((config) => {
  const token = localStorage.getItem(STORAGE_TOKEN_KEY)
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

async function unwrap<T>(promise: Promise<{ data: ApiEnvelope<T> }>) {
  const response = await promise
  return response.data.data
}

async function withDemoFallback<T>(request: () => Promise<T>, fallback: () => T | Promise<T>) {
  try {
    return await request()
  } catch (error) {
    if (ENABLE_DEMO_MODE) {
      return await fallback()
    }
    throw error
  }
}

function toPagedResult<T>(list: T[], pageNo = 1, pageSize = 10): PagedResult<T> {
  return {
    list,
    pageNo,
    pageSize,
    total: list.length,
    totalPages: Math.max(1, Math.ceil(list.length / pageSize)),
  }
}

function filterFiles(filters?: FileFilters) {
  return demoFiles.filter((item) => {
    const keywordMatched =
      !filters?.keyword || item.sourceFileName.toLowerCase().includes(filters.keyword.toLowerCase())
    const categoryMatched = !filters?.categoryId || item.categoryId === filters.categoryId
    const visibilityMatched = !filters?.visibility || item.visibility === filters.visibility
    const statusMatched = !filters?.generateStatus || item.generateTotalStatus === filters.generateStatus
    return keywordMatched && categoryMatched && visibilityMatched && statusMatched
  })
}

function filterTasks(filters?: TaskFilters) {
  return demoTasks.filter((item) => !filters?.status || item.status === filters.status)
}

function filterNotifications(filters?: NotificationFilters) {
  return demoNotifications.filter((item) => {
    const statusMatched = !filters?.status || item.status === filters.status
    const typeMatched = !filters?.type || item.type === filters.type
    return statusMatched && typeMatched
  })
}

function filterUsers(filters?: UserFilters) {
  return demoUsers.filter((item) => {
    const emailMatched = !filters?.email || item.email.includes(filters.email)
    const statusMatched = !filters?.status || item.status === filters.status
    return emailMatched && statusMatched
  })
}

function filterInviteCodes(filters?: InviteCodeFilters) {
  return demoInviteCodes.filter((item) => {
    const keywordMatched = !filters?.keyword || item.code.includes(filters.keyword)
    const statusMatched = !filters?.status || item.status === filters.status
    const batchMatched = !filters?.batchNo || item.batchNo === filters.batchNo
    return keywordMatched && statusMatched && batchMatched
  })
}

function withCaptchaPayload<T extends object>(payload: T, captchaData: GeetestValidateResult) {
  return {
    ...payload,
    lot_number: captchaData.lot_number,
    captcha_output: captchaData.captcha_output,
    pass_token: captchaData.pass_token,
    gen_time: captchaData.gen_time,
    captcha_id: captchaData.captcha_id,
  }
}

export const authApi = {
  async login(payload: {
    email: string
    password: string
    captchaData: GeetestValidateResult
  }) {
    return withDemoFallback(
      () =>
        unwrap<AuthResult>(
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
        ),
      async () => ({
        ...demoAuthResult,
        user: {
          ...demoAuthResult.user,
          email: payload.email || demoAuthResult.user.email,
        },
      }),
    )
  },

  async register(payload: {
    email: string
    emailCode: string
    password: string
    confirmPassword: string
    inviteCode: string
    captchaData: GeetestValidateResult
  }) {
    return withDemoFallback(
      () =>
        unwrap<AuthResult>(
          http.post(
            '/auth/register',
            withCaptchaPayload(
              {
                email: payload.email,
                emailCode: payload.emailCode,
                password: payload.password,
                confirmPassword: payload.confirmPassword,
                inviteCode: payload.inviteCode,
              },
              payload.captchaData,
            ),
          ),
        ),
      async () => ({
        ...demoAuthResult,
        user: {
          ...demoAuthResult.user,
          email: payload.email,
          role: 'USER' as const,
        },
      }),
    )
  },

  async sendRegisterCode(payload: {
    email: string
    captchaData: GeetestValidateResult
  }) {
    return withDemoFallback(
      () =>
        unwrap<{ expireSeconds: number; nextSendAfterSeconds: number }>(
          http.post(
            '/auth/register/email-code/send',
            withCaptchaPayload(
              {
                email: payload.email,
              },
              payload.captchaData,
            ),
          ),
        ),
      async () => ({ expireSeconds: 180, nextSendAfterSeconds: 60 }),
    )
  },

  async sendResetCode(payload: {
    email: string
    captchaData: GeetestValidateResult
  }) {
    return withDemoFallback(
      () =>
        unwrap<void>(
          http.post(
            '/auth/password-reset/email-code/send',
            withCaptchaPayload(
              {
                email: payload.email,
              },
              payload.captchaData,
            ),
          ),
        ),
      async () => undefined,
    )
  },

  async resetPassword(payload: {
    email: string
    emailCode: string
    newPassword: string
    confirmPassword: string
  }) {
    return withDemoFallback(
      () => unwrap<void>(http.post('/auth/password-reset/confirm', payload)),
      async () => undefined,
    )
  },

  async me() {
    return withDemoFallback(
      () => unwrap<UserProfile>(http.get('/auth/me')),
      async () => demoAuthResult.user,
    )
  },

  async changePassword(payload: {
    oldPassword: string
    newPassword: string
    confirmPassword: string
  }) {
    return withDemoFallback(
      () => unwrap<void>(http.post('/auth/password/change', payload)),
      async () => undefined,
    )
  },

  async logout() {
    return withDemoFallback(
      () => unwrap<void>(http.post('/auth/logout')),
      async () => undefined,
    )
  },
}

export const fileApi = {
  async listFiles(filters: FileFilters) {
    return withDemoFallback(
      () => unwrap<PagedResult<FileListItem>>(http.get('/files', { params: filters })),
      async () => toPagedResult(filterFiles(filters), filters.pageNo, filters.pageSize),
    )
  },

  async getFileDetail(fileId: number) {
    return withDemoFallback(
      () => unwrap<FileDetail>(http.get(`/files/${fileId}`)),
      async () => getDemoFileDetail(fileId),
    )
  },

  async getCategories() {
    return withDemoFallback(
      () => unwrap<FileCategory[]>(http.get('/file-categories')),
      async () => demoCategories,
    )
  },

  async upload(formData: FormData) {
    return withDemoFallback(
      () => unwrap<{ fileId: number; taskId: number; taskNo: string; taskStatus: string }>(http.post('/admin/files/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } })),
      async () => ({
        fileId: 1004,
        taskId: 8003,
        taskNo: 'GEN-20260517-0010',
        taskStatus: 'PROCESSING',
      }),
    )
  },

  async deleteFile(fileId: number, confirmText: string) {
    return withDemoFallback(
      () => unwrap<void>(http.delete(`/admin/files/${fileId}`, { data: { confirmText } })),
      async () => undefined,
    )
  },

  async previewSource(fileId: number) {
    return withDemoFallback(
      () => unwrap<PreviewInfo>(http.get(`/files/${fileId}/preview-source`)),
      async () => {
        const detail = getDemoFileDetail(fileId)
        if (detail.previewRecord.previewMode === 'CONVERT_TO_PDF' && detail.previewRecord.previewStatus === 'PROCESSING') {
          return {
            fileId,
            previewMode: 'CONVERT_TO_PDF' as const,
            previewStatus: 'PROCESSING' as const,
            previewUrl: null,
            expireAt: null,
            renderType: 'PDF_SCROLL' as const,
            downloadUrl: `/api/v1/files/${fileId}/download-source`,
            message: '预览文件正在生成中，请稍后刷新',
          }
        }

        const renderType: PreviewInfo['renderType'] =
          detail.sourceFileType === 'text/markdown' ? 'MARKDOWN_RENDER' : 'PDF_SCROLL'

        return {
          fileId,
          previewMode: detail.previewRecord.previewMode,
          previewStatus: 'SUCCESS' as const,
          previewUrl: detail.previewRecord.previewObjectUrl,
          expireAt: '2026-05-17 22:00:00.000',
          renderType,
          downloadUrl: `/api/v1/files/${fileId}/download-source`,
        }
      },
    )
  },

  async previewResult(fileId: number, itemType: TaskItemType) {
    return withDemoFallback(
      () => unwrap<PreviewInfo>(http.get(`/files/${fileId}/preview-result`, { params: { itemType } })),
      async () => {
        const item = getDemoFileDetail(fileId).generateRecord.items.find((entry) => entry.itemType === itemType)
        return {
          fileId,
          itemType,
          itemStatus: item?.itemStatus ?? 'FAIL',
          previewUrl: item?.resultObjectUrl ?? null,
          expireAt: item?.resultObjectUrl ? '2026-05-17 22:00:00.000' : null,
        }
      },
    )
  },

  async downloadSource(fileId: number) {
    return withDemoFallback(
      () => unwrap<{ url: string; expireAt: string }>(http.get(`/files/${fileId}/download-source`)),
      async () => ({ url: 'https://example.com/download/source', expireAt: '2026-05-17 22:00:00.000' }),
    )
  },

  async downloadResult(fileId: number, itemType: TaskItemType) {
    return withDemoFallback(
      () => unwrap<{ url: string; expireAt: string }>(http.get(`/files/${fileId}/download-result`, { params: { itemType } })),
      async () => ({ url: `https://example.com/download/${fileId}/${itemType.toLowerCase()}.html`, expireAt: '2026-05-17 22:00:00.000' }),
    )
  },
}

export const taskApi = {
  async listTasks(filters: TaskFilters) {
    return withDemoFallback(
      () => unwrap<PagedResult<TaskRecord>>(http.get('/tasks', { params: filters })),
      async () => toPagedResult(filterTasks(filters), filters.pageNo, filters.pageSize),
    )
  },

  async getTask(taskId: number) {
    return withDemoFallback(
      () => unwrap<TaskRecord>(http.get(`/tasks/${taskId}`)),
      async () => getDemoTask(taskId),
    )
  },

  async retryTaskItem(taskId: number, taskItemId: number) {
    return withDemoFallback(
      () => unwrap<void>(http.post(`/admin/tasks/${taskId}/items/${taskItemId}/retry`)),
      async () => undefined,
    )
  },
}

export const notificationApi = {
  async listNotifications(filters: NotificationFilters) {
    return withDemoFallback(
      () => unwrap<PagedResult<NotificationRecord>>(http.get('/notifications', { params: filters })),
      async () => toPagedResult(filterNotifications(filters), filters.pageNo, filters.pageSize),
    )
  },

  async getNotification(notificationId: number) {
    return withDemoFallback(
      () => unwrap<NotificationRecord>(http.get(`/notifications/${notificationId}`)),
      async () => ({ ...getDemoNotification(notificationId), status: 'READ' }),
    )
  },

  async markRead(notificationId: number) {
    return withDemoFallback(
      () => unwrap<void>(http.post(`/notifications/${notificationId}/read`)),
      async () => undefined,
    )
  },

  async markBatchRead(notificationIds: number[]) {
    return withDemoFallback(
      () => unwrap<void>(http.post('/notifications/read/batch', { notificationIds })),
      async () => undefined,
    )
  },

  async unreadCount() {
    return withDemoFallback(
      () => unwrap<{ unreadCount: number }>(http.get('/notifications/unread-count')),
      async () => ({ unreadCount: demoNotifications.filter((item) => item.status === 'UNREAD').length }),
    )
  },
}

export const adminApi = {
  async listUsers(filters: UserFilters) {
    return withDemoFallback(
      () => unwrap<PagedResult<UserProfile>>(http.get('/admin/users', { params: filters })),
      async () => toPagedResult(filterUsers(filters), filters.pageNo, filters.pageSize),
    )
  },

  async enableUser(userId: number) {
    return withDemoFallback(
      () => unwrap<void>(http.post(`/admin/users/${userId}/enable`)),
      async () => undefined,
    )
  },

  async disableUser(userId: number, remark: string) {
    return withDemoFallback(
      () => unwrap<void>(http.post(`/admin/users/${userId}/disable`, { remark })),
      async () => undefined,
    )
  },

  async listAdminFiles(filters: FileFilters) {
    return withDemoFallback(
      () => unwrap<PagedResult<FileListItem>>(http.get('/admin/files', { params: filters })),
      async () => toPagedResult(filterFiles(filters), filters.pageNo, filters.pageSize),
    )
  },

  async listInviteCodes(filters: InviteCodeFilters) {
    return withDemoFallback(
      () => unwrap<PagedResult<InviteCodeRecord>>(http.get('/admin/invite-codes', { params: filters })),
      async () => toPagedResult(filterInviteCodes(filters), filters.pageNo, filters.pageSize),
    )
  },

  async createInviteCode(payload: {
    codeType: 'CUSTOM' | 'RANDOM'
    code?: string
    totalQuota: number
    remark?: string
  }) {
    return withDemoFallback(
      () => unwrap<void>(http.post('/admin/invite-codes', payload)),
      async () => undefined,
    )
  },

  async batchGenerateInviteCodes(payload: {
    generateCount: number
    totalQuota: number
    remark?: string
    codeType: 'RANDOM'
  }) {
    return withDemoFallback(
      () => unwrap<{ batchNo: string; generateCount: number; codes: InviteCodeRecord[] }>(http.post('/admin/invite-codes/batch-generate', payload)),
      async () => ({
        batchNo: 'INV-20260517-0099',
        generateCount: payload.generateCount,
        codes: demoInviteCodes,
      }),
    )
  },

  async updateInviteRemark(inviteCodeId: number, remark: string) {
    return withDemoFallback(
      () => unwrap<void>(http.put(`/admin/invite-codes/${inviteCodeId}/remark`, { remark })),
      async () => undefined,
    )
  },

  async deleteInviteCode(inviteCodeId: number) {
    return withDemoFallback(
      () => unwrap<void>(http.delete(`/admin/invite-codes/${inviteCodeId}`)),
      async () => undefined,
    )
  },

  async createCategory(payload: { name: string; sortNo: number }) {
    return withDemoFallback(
      () => unwrap<void>(http.post('/admin/file-categories', payload)),
      async () => undefined,
    )
  },

  async updateCategory(categoryId: number, payload: { name: string; sortNo: number; status: 'ENABLED' | 'DISABLED' }) {
    return withDemoFallback(
      () => unwrap<void>(http.put(`/admin/file-categories/${categoryId}`, payload)),
      async () => undefined,
    )
  },

  async deleteCategory(categoryId: number) {
    return withDemoFallback(
      () => unwrap<void>(http.delete(`/admin/file-categories/${categoryId}`)),
      async () => undefined,
    )
  },
}
