import type {
  AuthResult,
  FileCategory,
  FileDetail,
  FileListItem,
  InviteCodeRecord,
  NotificationRecord,
  TaskRecord,
  UserProfile,
} from './types.ts'

const now = '2026-05-17 21:00:00.000'

export const demoUser: UserProfile = {
  id: 9001,
  email: 'admin@qq.com',
  role: 'ADMIN',
  status: 'ENABLED',
  registeredAt: '2026-05-01 09:30:00.000',
}

export const demoAuthResult: AuthResult = {
  token: 'demo-admin-token',
  expireAt: '2026-05-24 21:00:00.000',
  user: demoUser,
}

export const demoCategories: FileCategory[] = [
  { id: 1, name: '默认分类', sortNo: 0, status: 'ENABLED', isDefault: true },
  { id: 2, name: '操作系统', sortNo: 10, status: 'ENABLED' },
  { id: 3, name: '计算机网络', sortNo: 20, status: 'ENABLED' },
  { id: 4, name: '数据库原理', sortNo: 30, status: 'ENABLED' },
]

export const demoFiles: FileListItem[] = [
  {
    id: 1001,
    sourceFileName: '计算机网络重点复习.pdf',
    sourceFileType: 'application/pdf',
    sourceFileSize: 3_145_728,
    categoryId: 3,
    categoryName: '计算机网络',
    visibility: 'PUBLIC',
    uploadUserId: 9001,
    uploadUserEmail: 'admin@qq.com',
    uploadTime: '2026-05-16 20:30:00.000',
    generateTotalStatus: 'SUCCESS',
  },
  {
    id: 1002,
    sourceFileName: '操作系统章节串讲.pptx',
    sourceFileType: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    sourceFileSize: 8_507_112,
    categoryId: 2,
    categoryName: '操作系统',
    visibility: 'PRIVATE_ADMIN',
    uploadUserId: 9001,
    uploadUserEmail: 'admin@qq.com',
    uploadTime: '2026-05-15 18:10:00.000',
    generateTotalStatus: 'PARTIAL_SUCCESS',
  },
  {
    id: 1003,
    sourceFileName: '数据库系统概论-笔记.md',
    sourceFileType: 'text/markdown',
    sourceFileSize: 245_600,
    categoryId: 4,
    categoryName: '数据库原理',
    visibility: 'PUBLIC',
    uploadUserId: 9001,
    uploadUserEmail: 'admin@qq.com',
    uploadTime: '2026-05-14 16:20:00.000',
    generateTotalStatus: 'PROCESSING',
  },
]

export const demoFileDetails: FileDetail[] = [
  {
    ...demoFiles[0],
    sourceFileHash: 'sha256-net-001',
    sourceFileUrl: 'https://example.com/source/network.pdf',
    generateRecord: {
      totalStatus: 'SUCCESS',
      lastGeneratedAt: '2026-05-16 20:35:00.000',
      items: [
        { itemType: 'QUESTION', itemStatus: 'SUCCESS', resultObjectUrl: 'https://example.com/q1.html' },
        { itemType: 'KNOWLEDGE', itemStatus: 'SUCCESS', resultObjectUrl: 'https://example.com/k1.html' },
        { itemType: 'EXTENDED', itemStatus: 'SUCCESS', resultObjectUrl: 'https://example.com/e1.html' },
      ],
    },
    previewRecord: {
      previewMode: 'DIRECT',
      previewStatus: 'SUCCESS',
      previewObjectUrl: 'https://example.com/source/network.pdf',
    },
  },
  {
    ...demoFiles[1],
    sourceFileHash: 'sha256-os-001',
    sourceFileUrl: 'https://example.com/source/os.pptx',
    generateRecord: {
      totalStatus: 'PARTIAL_SUCCESS',
      lastGeneratedAt: '2026-05-15 18:18:00.000',
      items: [
        { itemType: 'QUESTION', itemStatus: 'SUCCESS', resultObjectUrl: 'https://example.com/q2.html' },
        { itemType: 'KNOWLEDGE', itemStatus: 'SUCCESS', resultObjectUrl: 'https://example.com/k2.html' },
        { itemType: 'EXTENDED', itemStatus: 'FAIL', resultObjectUrl: null },
      ],
    },
    previewRecord: {
      previewMode: 'CONVERT_TO_PDF',
      previewStatus: 'PROCESSING',
      previewObjectUrl: null,
    },
  },
  {
    ...demoFiles[2],
    sourceFileHash: 'sha256-db-001',
    sourceFileUrl: 'https://example.com/source/db.md',
    generateRecord: {
      totalStatus: 'PROCESSING',
      lastGeneratedAt: now,
      items: [
        { itemType: 'QUESTION', itemStatus: 'PROCESSING', resultObjectUrl: null },
        { itemType: 'KNOWLEDGE', itemStatus: 'PENDING', resultObjectUrl: null },
        { itemType: 'EXTENDED', itemStatus: 'PENDING', resultObjectUrl: null },
      ],
    },
    previewRecord: {
      previewMode: 'DIRECT',
      previewStatus: 'SUCCESS',
      previewObjectUrl: 'https://example.com/source/db.md',
    },
  },
]

export const demoTasks: TaskRecord[] = [
  {
    id: 8001,
    taskNo: 'GEN-20260517-0001',
    status: 'SUCCESS',
    triggerType: 'UPLOAD',
    fileSnapshotName: '计算机网络重点复习.pdf',
    fileDeletedSnapshot: false,
    startedAt: '2026-05-16 20:31:00.000',
    finishedAt: '2026-05-16 20:35:00.000',
    lastErrorMessage: null,
    reuseExisting: true,
    taskRemark: '复用旧结果，未重新生成',
    items: [
      { id: 1, itemType: 'QUESTION', status: 'SUCCESS', autoRetryCount: 0, manualRetryCount: 0, lastErrorMessage: null, resultObjectUrl: 'https://example.com/q1.html' },
      { id: 2, itemType: 'KNOWLEDGE', status: 'SUCCESS', autoRetryCount: 0, manualRetryCount: 0, lastErrorMessage: null, resultObjectUrl: 'https://example.com/k1.html' },
      { id: 3, itemType: 'EXTENDED', status: 'SUCCESS', autoRetryCount: 0, manualRetryCount: 0, lastErrorMessage: null, resultObjectUrl: 'https://example.com/e1.html' },
    ],
  },
  {
    id: 8002,
    taskNo: 'GEN-20260516-0009',
    status: 'PARTIAL_SUCCESS',
    triggerType: 'UPLOAD',
    fileSnapshotName: '操作系统章节串讲.pptx',
    fileDeletedSnapshot: false,
    startedAt: '2026-05-15 18:11:00.000',
    finishedAt: '2026-05-15 18:18:00.000',
    lastErrorMessage: '扩展题生成阶段模型调用超时',
    reuseExisting: false,
    taskRemark: null,
    items: [
      { id: 4, itemType: 'QUESTION', status: 'SUCCESS', autoRetryCount: 0, manualRetryCount: 0, lastErrorMessage: null, resultObjectUrl: 'https://example.com/q2.html' },
      { id: 5, itemType: 'KNOWLEDGE', status: 'SUCCESS', autoRetryCount: 0, manualRetryCount: 0, lastErrorMessage: null, resultObjectUrl: 'https://example.com/k2.html' },
      { id: 6, itemType: 'EXTENDED', status: 'FAIL', autoRetryCount: 3, manualRetryCount: 1, lastErrorMessage: '模型调用超时', resultObjectUrl: null },
    ],
  },
]

export const demoNotifications: NotificationRecord[] = [
  {
    id: 2001,
    title: '生成任务成功',
    summary: '《计算机网络重点复习.pdf》题目、知识点、扩展题已全部生成完成。',
    content: '本次任务已成功完成，你可以前往任务详情或文件详情页进行在线预览、下载源文件和 HTML 文件。',
    type: 'GENERATE_SUCCESS',
    status: 'UNREAD',
    createdAt: '2026-05-16 20:36:00.000',
    targetTaskId: 8001,
  },
  {
    id: 2002,
    title: '生成任务部分成功',
    summary: '《操作系统章节串讲.pptx》扩展题生成失败，请检查失败原因并按需重试。',
    content: '题目页和知识点页已生成完成，但扩展题 HTML 在模型调用阶段超时。你可以在我的任务页中单独重试失败子任务。',
    type: 'PARTIAL_SUCCESS',
    status: 'READ',
    createdAt: '2026-05-15 18:20:00.000',
    targetTaskId: 8002,
  },
]

export const demoUsers: UserProfile[] = [
  demoUser,
  { id: 10001, email: 'user1@qq.com', role: 'USER', status: 'ENABLED', registeredAt: '2026-05-12 10:20:00.000' },
  { id: 10002, email: 'user2@qq.com', role: 'USER', status: 'DISABLED', registeredAt: '2026-05-13 11:20:00.000' },
]

export const demoInviteCodes: InviteCodeRecord[] = [
  { id: 1, code: 'OS-2026-001', totalQuota: 20, remainingQuota: 11, remark: '操作系统课程', batchNo: 'INV-20260517-0001', status: 'ACTIVE' },
  { id: 2, code: 'NET-2026-002', totalQuota: 10, remainingQuota: 4, remark: '计算机网络期末', batchNo: 'INV-20260517-0002', status: 'ACTIVE' },
]

export function getDemoFileDetail(fileId: number) {
  return demoFileDetails.find((item) => item.id === fileId) ?? demoFileDetails[0]
}

export function getDemoTask(taskId: number) {
  return demoTasks.find((item) => item.id === taskId) ?? demoTasks[0]
}

export function getDemoNotification(notificationId: number) {
  return demoNotifications.find((item) => item.id === notificationId) ?? demoNotifications[0]
}
