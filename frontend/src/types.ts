export type UserRole = 'ADMIN' | 'USER'
export type UserStatus = 'ENABLED' | 'DISABLED'
export type Visibility = 'PUBLIC' | 'PRIVATE_ADMIN'
export type GenerateStatus = 'PENDING' | 'PROCESSING' | 'PARTIAL_SUCCESS' | 'SUCCESS' | 'FAIL'
export type TaskItemType = 'QUESTION' | 'KNOWLEDGE' | 'EXTENDED'
export type NotificationStatus = 'READ' | 'UNREAD'
export type NotificationType =
  | 'GENERATE_SUCCESS'
  | 'PARTIAL_SUCCESS'
  | 'GENERATE_FAIL'
  | 'PREVIEW_CONVERSION_SUCCESS'
  | 'PREVIEW_CONVERSION_FAIL'

export interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
  requestId: string
}

export interface UserProfile {
  id: number
  email: string
  role: UserRole
  status: UserStatus
  registeredAt?: string
}

export interface AuthResult {
  token: string
  expireAt: string
  user: UserProfile
}

export interface GeetestValidateResult {
  lot_number: string
  captcha_output: string
  pass_token: string
  gen_time: string
  captcha_id: string
}

export interface FileCategory {
  id: number
  name: string
  sortNo: number
  status: 'ENABLED' | 'DISABLED'
  isDefault?: boolean
}

export interface FileListItem {
  id: number
  sourceFileName: string
  sourceFileType: string
  sourceFileSize: number
  categoryId: number
  categoryName: string
  visibility: Visibility
  uploadUserId: number
  uploadUserEmail: string
  uploadTime: string
  generateTotalStatus: GenerateStatus
}

export interface GenerateRecordItem {
  itemType: TaskItemType
  itemStatus: GenerateStatus
  resultObjectUrl: string | null
}

export interface FileDetail extends FileListItem {
  sourceFileHash: string
  sourceFileUrl: string
  generateRecord: {
    totalStatus: GenerateStatus
    lastGeneratedAt: string
    items: GenerateRecordItem[]
  }
  previewRecord: {
    previewMode: 'DIRECT' | 'CONVERT_TO_PDF'
    previewStatus: 'SUCCESS' | 'PROCESSING' | 'FAIL'
    previewObjectUrl: string | null
  }
}

export interface PreviewInfo {
  fileId: number
  previewMode?: 'DIRECT' | 'CONVERT_TO_PDF'
  previewStatus?: 'SUCCESS' | 'PROCESSING' | 'FAIL'
  sourceFileType?: string
  previewUrl: string | null
  expireAt: string | null
  renderType?: 'PDF_SCROLL' | 'IMAGE_VERTICAL' | 'MARKDOWN_RENDER'
  downloadUrl?: string
  message?: string
  itemType?: TaskItemType
  itemStatus?: GenerateStatus
}

export interface TaskItem {
  id: number
  itemType: TaskItemType
  status: GenerateStatus
  autoRetryCount: number
  manualRetryCount: number
  lastErrorMessage: string | null
  resultObjectUrl: string | null
}

export interface TaskRecord {
  id: number
  taskNo: string
  status: GenerateStatus
  triggerType: 'UPLOAD' | 'RETRY'
  fileSnapshotName: string
  fileDeletedSnapshot: boolean
  startedAt: string
  finishedAt: string | null
  lastErrorMessage: string | null
  reuseExisting: boolean
  taskRemark: string | null
  items: TaskItem[]
}

export interface NotificationRecord {
  id: number
  title: string
  summary: string
  content: string
  type: NotificationType
  status: NotificationStatus
  createdAt: string
  targetTaskId: number | null
}

export interface InviteCodeRecord {
  id: number
  code: string
  totalQuota: number
  remainingQuota: number
  remark: string
  batchNo: string | null
  status: 'ACTIVE' | 'DISABLED'
}

export interface PagedResult<T> {
  list: T[]
  pageNo: number
  pageSize: number
  total: number
  totalPages: number
}

export interface FileFilters {
  pageNo?: number
  pageSize?: number
  keyword?: string
  categoryId?: number
  visibility?: Visibility
  generateStatus?: GenerateStatus
}

export interface TaskFilters {
  pageNo?: number
  pageSize?: number
  status?: GenerateStatus
}

export interface NotificationFilters {
  pageNo?: number
  pageSize?: number
  status?: NotificationStatus
  type?: NotificationType
}

export interface UserFilters {
  pageNo?: number
  pageSize?: number
  email?: string
  status?: UserStatus
}

export interface InviteCodeFilters {
  pageNo?: number
  pageSize?: number
  keyword?: string
  status?: 'ACTIVE' | 'DISABLED'
  batchNo?: string
}
