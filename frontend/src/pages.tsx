import {
  CheckCircleOutlined,
  ClockCircleOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EyeOutlined,
  LockOutlined,
  MailOutlined,
  SafetyCertificateOutlined,
  UserOutlined,
  PlusOutlined,
  ReloadOutlined,
  InboxOutlined,
} from '@ant-design/icons'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Alert,
  Button,
  Card,
  Col,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  List,
  message,
  Modal,
  Row,
  Select,
  Space,
  Statistic,
  Table,
  Tag,
  Typography,
  Upload,
} from 'antd'
import type { UploadFile } from 'antd'
import { useEffect, useMemo, useState } from 'react'
import { Navigate, useNavigate, useParams } from 'react-router-dom'

import { ApiError, adminApi, authApi, fileApi, notificationApi, resolveOpenableUrl, taskApi } from './api.ts'
import { useAuth } from './auth.tsx'
import { GeetestCaptchaPanel } from './geetest.tsx'
import type {
  BatchGenerateInviteCodesRequest,
  ChangePasswordRequest,
  CreateInviteCodeRequest,
  CreateCategoryRequest,
  FileCategory,
  FileFilters,
  FileListItem,
  GeetestValidateResult,
  GenerateStatus,
  InviteCodeRecord,
  InviteCodeFilters,
  LoginRequest,
  NotificationFilters,
  NotificationRecord,
  NotificationType,
  RegisterRequest,
  ResetPasswordRequest,
  RetryTaskItemParams,
  DisableUserRequest,
  TaskFilters,
  TaskItemType,
  UpdateCategoryRequest,
  UpdateInviteRemarkRequest,
  UserFilters,
  Visibility,
} from './types.ts'

const visibilityOptions = [
  { label: '公开', value: 'PUBLIC' },
  { label: '仅自己可见', value: 'PRIVATE_ADMIN' },
]

const statusLabelMap: Record<GenerateStatus, string> = {
  PENDING: '待处理',
  PROCESSING: '处理中',
  PARTIAL_SUCCESS: '部分成功',
  SUCCESS: '成功',
  FAIL: '失败',
}

const statusColorMap: Record<GenerateStatus, string> = {
  PENDING: 'default',
  PROCESSING: 'processing',
  PARTIAL_SUCCESS: 'warning',
  SUCCESS: 'success',
  FAIL: 'error',
}

const itemTypeLabelMap: Record<TaskItemType, string> = {
  QUESTION: '题目页',
  KNOWLEDGE: '知识点页',
  EXTENDED: '扩展题页',
}

const taskItemTypes: TaskItemType[] = ['QUESTION', 'KNOWLEDGE', 'EXTENDED']

const notificationTypeLabelMap: Record<NotificationType, string> = {
  GENERATE_SUCCESS: '生成成功',
  PARTIAL_SUCCESS: '部分成功',
  GENERATE_FAIL: '生成失败',
  PREVIEW_CONVERSION_SUCCESS: '预览转换成功',
  PREVIEW_CONVERSION_FAIL: '预览转换失败',
}

function formatBytes(size: number) {
  if (size < 1024) {
    return `${size} B`
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(1)} KB`
  }
  return `${(size / 1024 / 1024).toFixed(1)} MB`
}

async function openLink(url: string | null | undefined) {
  if (!url) {
    message.warning('当前资源尚未生成或暂不可预览')
    return
  }
  window.open(await resolveOpenableUrl(url), '_blank', 'noopener,noreferrer')
}

async function openRemoteLink(loader: () => Promise<string | null | undefined>) {
  try {
    await openLink(await loader())
  } catch {
    message.error('获取文件访问地址失败，请检查后端服务或存储配置')
  }
}

function isTaskItemType(value: string | undefined): value is TaskItemType {
  return taskItemTypes.includes((value ?? '').toUpperCase() as TaskItemType)
}

function getErrorMessage(error: unknown) {
  if (error instanceof ApiError) {
    return error.message
  }
  if (error instanceof Error) {
    return error.message
  }
  return '请求失败，请稍后重试'
}

function isRunningGenerateStatus(status: GenerateStatus | undefined) {
  return status === 'PENDING' || status === 'PROCESSING'
}

function StatusTag({ status }: { status: GenerateStatus }) {
  return <Tag color={statusColorMap[status]}>{statusLabelMap[status]}</Tag>
}

function VisibilityTag({ visibility }: { visibility: Visibility }) {
  return <Tag color={visibility === 'PUBLIC' ? 'blue' : 'purple'}>{visibility === 'PUBLIC' ? '公开' : '仅自己可见'}</Tag>
}

function PageHeaderCard(props: {
  title: string
  description?: string
  extra?: React.ReactNode
  children?: React.ReactNode
}) {
  return (
    <Card className="page-header-card" variant="borderless">
      <Row gutter={[24, 24]} justify="space-between" align="middle">
        <Col flex="auto">
          <Typography.Title level={3} style={{ marginTop: 0 }}>
            {props.title}
          </Typography.Title>
          {props.description && (
            <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
              {props.description}
            </Typography.Paragraph>
          )}
        </Col>
        {props.extra ? <Col>{props.extra}</Col> : null}
      </Row>
      {props.children ? <div className="page-header-card__content">{props.children}</div> : null}
    </Card>
  )
}

export function LoginPage() {
  const [loading, setLoading] = useState(false)
  const [captchaData, setCaptchaData] = useState<GeetestValidateResult | null>(null)
  const [captchaVersion, setCaptchaVersion] = useState(0)
  const navigate = useNavigate()
  const { login } = useAuth()

  const onFinish = async (values: Omit<LoginRequest, 'captchaData'>) => {
    if (!captchaData) {
      message.warning('请先完成安全验证')
      return
    }

    setLoading(true)
    try {
      await login({
        ...values,
        captchaData,
      })
      message.success('登录成功')
      navigate('/files', { replace: true })
    } catch (error) {
      message.error(getErrorMessage(error))
    } finally {
      setCaptchaData(null)
      setCaptchaVersion((value) => value + 1)
      setLoading(false)
    }
  }

  return (
    <div className="auth-form-container">
      <div className="auth-form-header">
        <Typography.Title level={2} className="auth-title">欢迎回来</Typography.Title>
        <Typography.Text type="secondary" className="auth-subtitle">登录您的账号以继续</Typography.Text>
      </div>
      <Form layout="vertical" onFinish={onFinish} size="large" className="auth-form">
        <Form.Item name="email" rules={[{ required: true }, { type: 'email' }]}>
          <Input placeholder="QQ 邮箱" prefix={<MailOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <Form.Item name="password" rules={[{ required: true }, { min: 8 }]}>
          <Input.Password placeholder="密码" prefix={<LockOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <GeetestCaptchaPanel
          key={captchaVersion}
          sceneLabel="登录提交"
          value={captchaData}
          onChange={setCaptchaData}
        />
        <Form.Item style={{ marginTop: 24, marginBottom: 12 }}>
          <Button type="primary" htmlType="submit" block loading={loading} className="auth-submit-btn">
            登录
          </Button>
        </Form.Item>
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
          <Button type="link" onClick={() => navigate('/forgot-password')} className="forgot-pwd-btn">忘记密码？</Button>
        </div>
        <div className="auth-form-footer" style={{ marginTop: 0 }}>
          还没有账号？ <Button type="link" onClick={() => navigate('/register')} className="auth-link-btn">立即注册</Button>
        </div>
      </Form>
    </div>
  )
}

export function RegisterPage() {
  const [sending, setSending] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [captchaData, setCaptchaData] = useState<GeetestValidateResult | null>(null)
  const [captchaVersion, setCaptchaVersion] = useState(0)
  const [form] = Form.useForm()
  const navigate = useNavigate()
  const { register } = useAuth()

  const sendCode = async () => {
    const email = form.getFieldValue('email')
    if (!email) {
      message.warning('请先输入邮箱')
      return
    }
    if (!captchaData) {
      message.warning('请先完成安全验证')
      return
    }
    setSending(true)
    try {
      const result = await authApi.sendRegisterCode({ email, captchaData })
      message.success(`验证码已发送，${Math.floor(result.expireSeconds / 60)} 分钟内有效`)
    } catch (error) {
      message.error(getErrorMessage(error))
    } finally {
      setCaptchaData(null)
      setCaptchaVersion((value) => value + 1)
      setSending(false)
    }
  }

  const onFinish = async (values: RegisterRequest) => {
    setSubmitting(true)
    try {
      await register(values)
      message.success('注册成功，已自动登录')
      navigate('/files', { replace: true })
    } catch (error) {
      message.error(getErrorMessage(error))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-form-container">
      <div className="auth-form-header">
        <Typography.Title level={2} className="auth-title">创建账号</Typography.Title>
        <Typography.Text type="secondary" className="auth-subtitle">开启您的期末复习之旅</Typography.Text>
      </div>
      <Form form={form} layout="vertical" onFinish={onFinish} size="large" className="auth-form">
        <Form.Item name="email" rules={[{ required: true }, { type: 'email' }]}>
          <Input placeholder="QQ 邮箱" prefix={<MailOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <GeetestCaptchaPanel key={captchaVersion} sceneLabel="注册" value={captchaData} onChange={setCaptchaData} />
        <Form.Item required style={{ marginBottom: 24, marginTop: 24 }}>
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item name="emailCode" noStyle rules={[{ required: true }]}>
              <Input placeholder="6 位验证码" prefix={<SafetyCertificateOutlined className="auth-input-icon" />} className="auth-input" style={{ width: 'calc(100% - 120px)' }} />
            </Form.Item>
            <Button loading={sending} onClick={sendCode} style={{ width: 120, height: 48, borderRadius: '0 12px 12px 0' }}>
              获取验证码
            </Button>
          </Space.Compact>
        </Form.Item>
        <Form.Item name="inviteCode" rules={[{ required: true }]}>
          <Input placeholder="邀请码" prefix={<UserOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <Form.Item name="password" rules={[{ required: true }, { min: 8 }]}>
          <Input.Password placeholder="密码 (至少 8 位)" prefix={<LockOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <Form.Item
          name="confirmPassword"
          dependencies={['password']}
          rules={[
            { required: true },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('password') === value) {
                  return Promise.resolve()
                }
                return Promise.reject(new Error('两次输入的密码不一致'))
              },
            }),
          ]}
        >
          <Input.Password placeholder="确认密码" prefix={<LockOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <Form.Item style={{ marginTop: 24 }}>
          <Button type="primary" htmlType="submit" block loading={submitting} className="auth-submit-btn">
            注册并登录
          </Button>
        </Form.Item>
        <div className="auth-form-footer">
          已有账号？ <Button type="link" onClick={() => navigate('/login')} className="auth-link-btn">直接登录</Button>
        </div>
      </Form>
    </div>
  )
}

export function ForgotPasswordPage() {
  const [sending, setSending] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [captchaData, setCaptchaData] = useState<GeetestValidateResult | null>(null)
  const [captchaVersion, setCaptchaVersion] = useState(0)
  const [form] = Form.useForm()
  const navigate = useNavigate()

  const sendCode = async () => {
    const email = form.getFieldValue('email')
    if (!email) {
      message.warning('请先输入邮箱')
      return
    }
    if (!captchaData) {
      message.warning('请先完成安全验证')
      return
    }
    setSending(true)
    try {
      await authApi.sendResetCode({ email, captchaData })
      message.success('重置验证码已发送')
      message.info('如需再次发送，请重新完成安全验证')
    } catch (error) {
      message.error(getErrorMessage(error))
    } finally {
      setCaptchaData(null)
      setCaptchaVersion((value) => value + 1)
      setSending(false)
    }
  }

  const onFinish = async (values: ResetPasswordRequest) => {
    setSubmitting(true)
    try {
      await authApi.resetPassword(values)
      message.success('密码已重置，请重新登录')
      navigate('/login', { replace: true })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="auth-form-container">
      <div className="auth-form-header">
        <Typography.Title level={2} className="auth-title">重置密码</Typography.Title>
        <Typography.Text type="secondary" className="auth-subtitle">验证邮箱以设置新密码</Typography.Text>
      </div>
      <Form form={form} layout="vertical" onFinish={onFinish} size="large" className="auth-form">
        <Form.Item name="email" rules={[{ required: true }, { type: 'email' }]}>
          <Input placeholder="QQ 邮箱" prefix={<MailOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <GeetestCaptchaPanel
          key={captchaVersion}
          sceneLabel="发送重置验证码"
          value={captchaData}
          onChange={setCaptchaData}
        />
        <Form.Item required style={{ marginBottom: 24, marginTop: 24 }}>
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item name="emailCode" noStyle rules={[{ required: true }]}>
              <Input placeholder="6 位验证码" prefix={<SafetyCertificateOutlined className="auth-input-icon" />} className="auth-input" style={{ width: 'calc(100% - 120px)' }} />
            </Form.Item>
            <Button loading={sending} onClick={sendCode} style={{ width: 120, height: 48, borderRadius: '0 12px 12px 0' }}>
              获取验证码
            </Button>
          </Space.Compact>
        </Form.Item>
        <Form.Item name="newPassword" rules={[{ required: true }, { min: 8 }]}>
          <Input.Password placeholder="新密码 (至少 8 位)" prefix={<LockOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <Form.Item
          name="confirmPassword"
          dependencies={['newPassword']}
          rules={[
            { required: true },
            ({ getFieldValue }) => ({
              validator(_, value) {
                if (!value || getFieldValue('newPassword') === value) {
                  return Promise.resolve()
                }
                return Promise.reject(new Error('两次输入的密码不一致'))
              },
            }),
          ]}
        >
          <Input.Password placeholder="确认新密码" prefix={<LockOutlined className="auth-input-icon" />} className="auth-input" />
        </Form.Item>
        <Form.Item style={{ marginTop: 24 }}>
          <Button type="primary" htmlType="submit" block loading={submitting} className="auth-submit-btn">
            重置密码
          </Button>
        </Form.Item>
        <div className="auth-form-footer">
          记起密码了？ <Button type="link" onClick={() => navigate('/login')} className="auth-link-btn">返回登录</Button>
        </div>
      </Form>
    </div>
  )
}

export function ChangePasswordPage() {
  const [form] = Form.useForm()
  const [loading, setLoading] = useState(false)
  const { logout } = useAuth()
  const navigate = useNavigate()

  const onFinish = async (values: ChangePasswordRequest) => {
    setLoading(true)
    try {
      await authApi.changePassword(values)
      message.success('密码修改成功，请重新登录')
      await logout()
      navigate('/login', { replace: true })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="page-stack">
      <PageHeaderCard title="修改密码" description="已登录用户通过旧密码修改密码，成功后旧登录态立即失效。" />
      <div className="settings-container">
        <Card variant="borderless" className="settings-card">
          <Form form={form} layout="vertical" onFinish={onFinish} size="large">
            <Form.Item label="旧密码" name="oldPassword" rules={[{ required: true }]}>
              <Input.Password placeholder="请输入旧密码" className="settings-input" />
            </Form.Item>
            <Form.Item label="新密码" name="newPassword" rules={[{ required: true }, { min: 8 }]}>
              <Input.Password placeholder="请输入新密码" className="settings-input" />
            </Form.Item>
            <Form.Item
              label="确认新密码"
              name="confirmPassword"
              dependencies={['newPassword']}
              rules={[
                { required: true },
                ({ getFieldValue }) => ({
                  validator(_, value) {
                    if (!value || getFieldValue('newPassword') === value) {
                      return Promise.resolve()
                    }
                    return Promise.reject(new Error('两次输入的密码不一致'))
                  },
                }),
              ]}
            >
              <Input.Password placeholder="请再次输入新密码" className="settings-input" />
            </Form.Item>
            <Form.Item style={{ marginTop: 32, marginBottom: 0 }}>
              <Button type="primary" htmlType="submit" loading={loading} className="settings-submit-btn">
                确认修改
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </div>
    </div>
  )
}

export function FileListPage() {
  const navigate = useNavigate()
  const { isAdmin } = useAuth()
  const [filters, setFilters] = useState<FileFilters>({ pageNo: 1, pageSize: 10 })
  const categoriesQuery = useQuery({ queryKey: ['file-categories'], queryFn: fileApi.getCategories })
  const filesQuery = useQuery({
    queryKey: ['files', filters],
    queryFn: () => fileApi.listFiles(filters),
  })

  const categoryOptions = useMemo(
    () => (categoriesQuery.data ?? []).map((item) => ({ label: item.name, value: item.id })),
    [categoriesQuery.data],
  )

  const previewMutation = useMutation({
    mutationFn: async (fileId: number) => fileApi.previewSource(fileId),
    onSuccess: (data) => void openLink(data.previewUrl),
    onError: () => message.error('获取预览地址失败，请检查后端服务或存储配置'),
  })

  return (
    <div className="page-stack">
      <PageHeaderCard
        title="文件列表"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => void filesQuery.refetch()}>
              刷新
            </Button>
            {isAdmin ? (
              <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/upload')}>
                上传文件
              </Button>
            ) : null}
          </Space>
        }
      >
        <Space wrap size="middle">
          <div className="stat-card">
            <Statistic title="当前文件数" value={filesQuery.data?.total ?? 0} />
          </div>
          <div className="stat-card">
            <Statistic title="公开文件" value={(filesQuery.data?.list ?? []).filter((item) => item.visibility === 'PUBLIC').length} />
          </div>
          <div className="stat-card">
            <Statistic title="处理中" value={(filesQuery.data?.list ?? []).filter((item) => item.generateTotalStatus === 'PROCESSING').length} />
          </div>
          <div className="stat-card">
            <Statistic title="已成功" value={(filesQuery.data?.list ?? []).filter((item) => item.generateTotalStatus === 'SUCCESS').length} />
          </div>
        </Space>
      </PageHeaderCard>
      <Card variant="borderless" className="data-table-card">
        {filesQuery.isError ? (
          <Alert
            type="error"
            showIcon
            message="文件列表加载失败"
            description="当前显示的数据不再回退到演示结果，请检查后端接口状态后重试。"
            style={{ marginBottom: 16 }}
          />
        ) : null}
        <div className="filter-bar">
          <Form layout="vertical" onFinish={(values) => setFilters({ ...filters, ...values })}>
            <Row gutter={[16, 8]} align="bottom">
              <Col xs={24} md={8}>
                <Form.Item label="文件名搜索" name="keyword">
                  <Input placeholder="输入文件名关键字" size="large" />
                </Form.Item>
              </Col>
              <Col xs={24} md={5}>
                <Form.Item label="分类" name="categoryId">
                  <Select allowClear placeholder="全部分类" options={categoryOptions} size="large" />
                </Form.Item>
              </Col>
              <Col xs={24} md={5}>
                <Form.Item label="可见范围" name="visibility">
                  <Select allowClear placeholder="全部范围" options={visibilityOptions} size="large" />
                </Form.Item>
              </Col>
              <Col xs={24} md={4}>
                <Form.Item label="生成状态" name="generateStatus">
                  <Select
                    allowClear
                    placeholder="全部状态"
                    options={Object.entries(statusLabelMap).map(([value, label]) => ({ label, value }))}
                    size="large"
                  />
                </Form.Item>
              </Col>
              <Col xs={24} md={2} className="form-action-col">
                <Button type="primary" htmlType="submit" block size="large">
                  查询
                </Button>
              </Col>
            </Row>
          </Form>
        </div>
        <Table<FileListItem>
          rowKey="id"
          loading={filesQuery.isLoading || previewMutation.isPending}
          dataSource={filesQuery.data?.list}
          pagination={false}
          scroll={{ x: 800 }}
          columns={[
            { 
              title: '文件名', 
              dataIndex: 'sourceFileName', 
              width: 260,
              render: (text: string) => (
                <Typography.Text 
                  style={{ maxWidth: 220, margin: 0, fontSize: 16 }} 
                  ellipsis={{ tooltip: text }}
                >
                  {text}
                </Typography.Text>
              )
            },
            { title: '分类', dataIndex: 'categoryName', width: 140 },
            { title: '大小', dataIndex: 'sourceFileSize', width: 100, render: (value: number) => formatBytes(value) },
            {
              title: '操作',
              key: 'actions',
              fixed: 'right',
              width: 260,
              render: (_, record) => (
                <Space wrap>
                  <Button type="primary" size="small" onClick={() => navigate(`/files/${record.id}`)}>
                    详情
                  </Button>
                  <Button size="small" type="default" icon={<EyeOutlined />} onClick={() => previewMutation.mutate(record.id)}>
                    预览源文件
                  </Button>
                  <Button
                    size="small"
                    type="dashed"
                    icon={<DownloadOutlined />}
                    onClick={() => void openRemoteLink(async () => (await fileApi.downloadSource(record.id)).url)}
                  >
                    下载
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>
    </div>
  )
}

export function FileDetailPage() {
  const navigate = useNavigate()
  const params = useParams()
  const fileId = Number(params.fileId)
  const detailQuery = useQuery({
    queryKey: ['file-detail', fileId],
    queryFn: () => fileApi.getFileDetail(fileId),
    enabled: Number.isFinite(fileId),
  })

  if (!Number.isFinite(fileId)) {
    return <Navigate to="/files" replace />
  }

  const record = detailQuery.data

  return (
    <div className="page-stack">
      {detailQuery.isError ? (
        <Card variant="borderless">
          <Alert
            type="error"
            showIcon
            message="文件详情加载失败"
            description="当前未再使用演示数据回退，请检查后端接口和登录态。"
          />
        </Card>
      ) : null}
      {!record ? (
        <Card variant="borderless">
          <Empty description="未找到文件详情" />
        </Card>
      ) : (
        <>
          <div className="file-info-grid">
            <div className="file-info-item full-width">
              <div className="file-info-label">文件名</div>
              <div className="file-info-value large">{record.sourceFileName}</div>
            </div>
            <div className="file-info-item">
              <div className="file-info-label">分类</div>
              <div className="file-info-value">{record.categoryName}</div>
            </div>
            <div className="file-info-item">
              <div className="file-info-label">上传者</div>
              <div className="file-info-value">{record.uploadUserEmail}</div>
            </div>
            <div className="file-info-item">
              <div className="file-info-label">可见范围</div>
              <div className="file-info-value"><VisibilityTag visibility={record.visibility} /></div>
            </div>
            <div className="file-info-item">
              <div className="file-info-label">生成状态</div>
              <div className="file-info-value"><StatusTag status={record.generateRecord.totalStatus} /></div>
            </div>
            <div className="file-info-item">
              <div className="file-info-label">文件大小</div>
              <div className="file-info-value">{formatBytes(record.sourceFileSize)}</div>
            </div>
            <div className="file-info-item">
              <div className="file-info-label">上传时间</div>
              <div className="file-info-value">{record.uploadTime}</div>
            </div>
            <div className="file-info-item">
              <div className="file-info-label">文件哈希</div>
              <div className="file-info-value mono">{record.sourceFileHash}</div>
            </div>
            <div className="file-info-item">
              <div className="file-info-label">预览方式</div>
              <div className="file-info-value"><Tag color="success" style={{ margin: 0 }}>直接在线预览</Tag></div>
            </div>
          </div>
          <Card
            title="生成结果与源文件"
            variant="borderless"
            className="data-table-card"
            extra={
              <Space>
                <Button
                  icon={<EyeOutlined />}
                  onClick={() => void openRemoteLink(async () => (await fileApi.previewSource(record.id)).previewUrl)}
                >
                  在线预览源文件
                </Button>
                <Button
                  icon={<DownloadOutlined />}
                  onClick={() => void openRemoteLink(async () => (await fileApi.downloadSource(record.id)).url)}
                >
                  下载源文件
                </Button>
              </Space>
            }
          >
            <List
              dataSource={record.generateRecord.items}
              renderItem={(item) => (
                <List.Item
                  actions={[
                    <Button key="preview" size="small" type="primary" ghost onClick={() => navigate(`/files/${record.id}/results/${item.itemType}`)}>
                      前端查看
                    </Button>,
                    <Button key="download" size="small" onClick={() => void openRemoteLink(async () => (await fileApi.downloadResult(record.id, item.itemType)).url)}>
                      下载 HTML
                    </Button>,
                  ]}
                >
                  <List.Item.Meta
                    avatar={item.itemStatus === 'SUCCESS' ? <CheckCircleOutlined /> : <ClockCircleOutlined />}
                    title={itemTypeLabelMap[item.itemType]}
                    description={
                      <Space orientation="vertical" size={8} style={{ marginTop: 8 }}>
                        <StatusTag status={item.itemStatus} />
                      </Space>
                    }
                  />
                </List.Item>
              )}
            />
          </Card>
        </>
      )}
    </div>
  )
}

export function GeneratedHtmlPreviewPage() {
  const navigate = useNavigate()
  const params = useParams()
  const fileId = Number(params.fileId)
  const itemTypeParam = params.itemType?.toUpperCase()
  const itemType = isTaskItemType(itemTypeParam) ? itemTypeParam : null

  const detailQuery = useQuery({
    queryKey: ['file-detail', fileId],
    queryFn: () => fileApi.getFileDetail(fileId),
    enabled: Number.isFinite(fileId),
  })

  const previewQuery = useQuery({
    queryKey: ['file-result-preview', fileId, itemType],
    queryFn: () => fileApi.previewResult(fileId, itemType!),
    enabled: Number.isFinite(fileId) && itemType !== null,
  })

  const htmlQuery = useQuery({
    queryKey: ['file-result-html', fileId, itemType],
    queryFn: () => fileApi.previewResultHtml(fileId, itemType!),
    enabled: Number.isFinite(fileId) && itemType !== null && Boolean(previewQuery.data?.previewUrl),
    retry: false,
  })

  if (!Number.isFinite(fileId)) {
    return <Navigate to="/files" replace />
  }

  if (itemType === null) {
    return <Navigate to={`/files/${fileId}`} replace />
  }

  const previewData = previewQuery.data

  return (
    <div className="page-stack">
      <PageHeaderCard
        title={`${detailQuery.data?.sourceFileName ?? '文件'} / ${itemTypeLabelMap[itemType]}`}
        description="生成后的 HTML 已放入前端路由页面，可直接在当前界面内访问和预览。"
        extra={
          <Space wrap>
            <Button onClick={() => navigate(`/files/${fileId}`)}>返回文件详情</Button>
            <Button icon={<ReloadOutlined />} onClick={() => void previewQuery.refetch()}>
              刷新结果
            </Button>
            <Button
              type="primary"
              icon={<DownloadOutlined />}
              onClick={() => void openRemoteLink(async () => (await fileApi.downloadResult(fileId, itemType)).url)}
            >
              下载 HTML
            </Button>
          </Space>
        }
      >
        <Space wrap>
          {taskItemTypes.map((entry) => (
            <Button
              key={entry}
              type={entry === itemType ? 'primary' : 'default'}
              onClick={() => navigate(`/files/${fileId}/results/${entry}`)}
            >
              {itemTypeLabelMap[entry]}
            </Button>
          ))}
        </Space>
      </PageHeaderCard>

      <Card variant="borderless" loading={detailQuery.isLoading || previewQuery.isLoading || htmlQuery.isLoading}>
        {htmlQuery.data ? (
          <iframe
            title={`${detailQuery.data?.sourceFileName ?? '文件'}-${itemType}`}
            srcDoc={htmlQuery.data}
            style={{ width: '100%', minHeight: 820, border: 0, borderRadius: 12, background: '#fff' }}
          />
        ) : htmlQuery.isError ? (
          <Alert
            type="error"
            showIcon
            message={`${itemTypeLabelMap[itemType]}加载失败`}
            description="已获取到结果记录，但前端内嵌渲染失败。你仍可先使用“下载 HTML”验证生成内容。"
          />
        ) : (
          <Alert
            type="info"
            showIcon
            message={`${itemTypeLabelMap[itemType]}暂时不可访问`}
            description={
              previewData?.itemStatus === 'PROCESSING'
                ? '当前 HTML 正在生成中，请稍后点击“刷新结果”重新查看。'
                : previewData?.itemStatus === 'FAIL'
                  ? '当前 HTML 生成失败，请回到任务页或文件详情页查看状态后重试。'
                  : '当前 HTML 还未生成完成，请稍后再试。'
            }
          />
        )}
      </Card>
    </div>
  )
}

export function UploadPage() {
  const [form] = Form.useForm()
  const navigate = useNavigate()
  const categoriesQuery = useQuery({ queryKey: ['file-categories'], queryFn: fileApi.getCategories })
  const uploadMutation = useMutation({
    mutationFn: (formData: FormData) => fileApi.upload(formData),
    onSuccess: (data) => {
      message.success(`上传成功，任务号：${data.taskNo}`)
      navigate('/tasks')
    },
  })

  const onFinish = async (values: {
    categoryId: number
    visibility: Visibility
    fileList: UploadFile[]
  }) => {
    const file = values.fileList?.[0]?.originFileObj
    if (!file) {
      message.warning('请先选择源文件')
      return
    }
    const formData = new FormData()
    formData.append('file', file)
    formData.append('categoryId', String(values.categoryId))
    formData.append('visibility', values.visibility)
    uploadMutation.mutate(formData)
  }

  return (
    <div className="page-stack">
      <Form form={form} layout="vertical" onFinish={onFinish} size="large">
        <Card variant="borderless" className="upload-workspace">
          <div className="upload-workspace__header">
            <div>
              <Typography.Title level={3} className="upload-workspace__title">
                上传文件
              </Typography.Title>
              <Typography.Text className="upload-workspace__subtitle">
                选择源文件并完成配置后提交
              </Typography.Text>
            </div>
          </div>
          <Row gutter={[28, 28]} align="stretch">
            <Col xs={24} xl={9}>
              <div className="upload-sidebar">
                <div className="upload-config-card">
                  <div className="upload-config-card__hero">
                    <div className="upload-config-card__eyebrow">配置面板</div>
                    <Typography.Title level={4} style={{ margin: 0 }}>
                      设置上传参数
                    </Typography.Title>
                    <Typography.Text type="secondary">完成下面两项后即可提交</Typography.Text>
                  </div>

                  <div className="upload-config-card__body">
                    <div className="upload-config-section">
                      <div className="upload-config-section__label">文件分类</div>
                      <Form.Item name="categoryId" rules={[{ required: true }]} style={{ marginBottom: 0 }}>
                        <Select
                          className="upload-config-select"
                          size="large"
                          placeholder="请选择分类"
                          options={(categoriesQuery.data ?? []).map((item: FileCategory) => ({
                            label: item.name,
                            value: item.id,
                          }))}
                        />
                      </Form.Item>
                    </div>

                    <div className="upload-config-section">
                      <div className="upload-config-section__label">可见范围</div>
                      <Form.Item name="visibility" rules={[{ required: true }]} style={{ marginBottom: 0 }}>
                        <Select className="upload-config-select" size="large" placeholder="请选择可见范围" options={visibilityOptions} />
                      </Form.Item>
                    </div>
                  </div>

                  <div className="upload-config-card__footer">
                    <div className="upload-config-card__hint">提交后立即创建任务</div>
                    <Button type="primary" htmlType="submit" loading={uploadMutation.isPending} className="settings-submit-btn">
                      确认上传
                    </Button>
                  </div>
                </div>
              </div>
            </Col>
            <Col xs={24} xl={15}>
              <div className="upload-stage-card">
                <Form.Item
                  name="fileList"
                  valuePropName="fileList"
                  getValueFromEvent={(event) => event?.fileList ?? []}
                  rules={[{ required: true, message: '请选择要上传的文件' }]}
                  style={{ margin: 0 }}
                >
                  <Upload.Dragger beforeUpload={() => false} maxCount={1} className="upload-dragger-massive">
                    <div className="upload-icon-wrapper">
                      <InboxOutlined className="upload-icon" />
                    </div>
                    <div className="upload-text">点击或拖拽上传</div>
                    <div className="upload-stage-support">支持 `PDF / Word / PPT / 图片 / TXT / Markdown`</div>
                  </Upload.Dragger>
                </Form.Item>
              </div>
            </Col>
          </Row>
        </Card>
      </Form>
    </div>
  )
}

export function TaskListPage() {
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState<TaskFilters>({ pageNo: 1, pageSize: 10 })
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null)
  const tasksQuery = useQuery({
    queryKey: ['tasks', filters],
    queryFn: () => taskApi.listTasks(filters),
    refetchInterval: (query) => {
      const list = query.state.data?.list ?? []
      return list.some((item) => isRunningGenerateStatus(item.status)) ? 5000 : false
    },
  })
  const taskDetailQuery = useQuery({
    queryKey: ['task-detail', selectedTaskId],
    queryFn: () => taskApi.getTask(selectedTaskId ?? 0),
    enabled: selectedTaskId !== null,
    refetchInterval: (query) => (isRunningGenerateStatus(query.state.data?.status) ? 3000 : false),
  })

  const retryMutation = useMutation({
    mutationFn: (params: RetryTaskItemParams) => taskApi.retryTaskItem(params),
    onSuccess: async () => {
      message.success('失败子任务已提交重试')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['tasks'] }),
        queryClient.invalidateQueries({ queryKey: ['task-detail', selectedTaskId] }),
      ])
    },
  })

  return (
    <div className="page-stack">
      <PageHeaderCard
        title="我的任务"
        description="第一版仅管理员可访问，支持查看总任务状态、子任务状态、自动重试次数以及手动重试失败项。"
      />
      <Card variant="borderless" className="data-table-card">
        {tasksQuery.isError ? (
          <Alert
            type="error"
            showIcon
            message="任务列表加载失败"
            description="当前未再回退到演示任务数据，请检查后端任务接口。"
            style={{ marginBottom: 16 }}
          />
        ) : null}
        <div className="filter-bar">
          <Form layout="inline" onFinish={(values) => setFilters({ ...filters, ...values })}>
            <Form.Item label="状态" name="status">
              <Select
                allowClear
                style={{ width: 180 }}
                placeholder="全部状态"
                options={Object.entries(statusLabelMap).map(([value, label]) => ({ label, value }))}
                size="large"
              />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" size="large">
                筛选
              </Button>
            </Form.Item>
            <Form.Item>
              <Button onClick={() => void tasksQuery.refetch()} icon={<ReloadOutlined />} size="large">
                刷新
              </Button>
            </Form.Item>
          </Form>
        </div>
        <Table
          style={{ marginTop: 16 }}
          rowKey="id"
          loading={tasksQuery.isLoading}
          dataSource={tasksQuery.data?.list}
          pagination={false}
          columns={[
            { title: '任务号', dataIndex: 'taskNo', width: 200 },
            { title: '文件快照', dataIndex: 'fileSnapshotName', width: 220 },
            { title: '状态', dataIndex: 'status', width: 120, render: (value: GenerateStatus) => <StatusTag status={value} /> },
            { title: '触发方式', dataIndex: 'triggerType', width: 120 },
            {
              title: '结果复用',
              dataIndex: 'reuseExisting',
              width: 120,
              render: (value: boolean) => (value ? <Tag color="blue">复用旧结果</Tag> : <Tag>新生成</Tag>),
            },
            { title: '开始时间', dataIndex: 'startedAt', width: 180 },
            {
              title: '操作',
              key: 'actions',
              width: 140,
              render: (_, record: { id: number }) => (
                <Button size="small" onClick={() => setSelectedTaskId(record.id)}>
                  查看详情
                </Button>
              ),
            },
          ]}
        />
      </Card>
      <Drawer
        open={selectedTaskId !== null}
        width={720}
        title={<span style={{ fontSize: 20, fontWeight: 700 }}>任务详情</span>}
        onClose={() => setSelectedTaskId(null)}
        className="task-detail-drawer"
        styles={{
          header: { borderBottom: '1px solid #f1f5f9', padding: '20px 24px' },
          body: { padding: '24px', background: '#f8fafc' },
        }}
      >
        {!taskDetailQuery.data ? (
          <Empty description="暂无任务详情" />
        ) : (
          <Space orientation="vertical" style={{ width: '100%' }} size="large">
            <Card variant="borderless" className="settings-card" style={{ padding: 12 }}>
              <Descriptions bordered column={1}>
                <Descriptions.Item label="任务号">{taskDetailQuery.data.taskNo}</Descriptions.Item>
                <Descriptions.Item label="状态">
                  <StatusTag status={taskDetailQuery.data.status} />
                </Descriptions.Item>
                <Descriptions.Item label="说明">
                  {taskDetailQuery.data.taskRemark ?? taskDetailQuery.data.lastErrorMessage ?? '无'}
                </Descriptions.Item>
              </Descriptions>
            </Card>
            <div style={{ marginBottom: 8 }}>
              <Typography.Title level={5} style={{ margin: '0 0 16px 4px', fontWeight: 600 }}>子任务执行列表</Typography.Title>
              <List
                dataSource={taskDetailQuery.data.items}
                renderItem={(item) => (
                  <List.Item
                    actions={[
                      item.status === 'FAIL' ? (
                        <Button
                          key="retry"
                          type="primary"
                          danger
                          ghost
                          loading={retryMutation.isPending}
                          onClick={() =>
                            retryMutation.mutate({
                              taskId: taskDetailQuery.data!.id,
                              taskItemId: item.id,
                            })
                          }
                        >
                          重试失败项
                        </Button>
                      ) : null,
                    ]}
                  >
                    <List.Item.Meta
                      avatar={item.status === 'SUCCESS' ? <CheckCircleOutlined /> : <ClockCircleOutlined />}
                      title={`${itemTypeLabelMap[item.itemType]} (自动重试 ${item.autoRetryCount} 次 / 手动 ${item.manualRetryCount} 次)`}
                      description={
                        <Space orientation="vertical" size={8} style={{ marginTop: 8 }}>
                          <StatusTag status={item.status} />
                          {item.lastErrorMessage ? <Typography.Text type="secondary" style={{ fontSize: 13, background: '#f1f5f9', padding: '4px 8px', borderRadius: 6, display: 'inline-block' }}>{item.lastErrorMessage}</Typography.Text> : null}
                        </Space>
                      }
                    />
                  </List.Item>
                )}
              />
            </div>
          </Space>
        )}
      </Drawer>
    </div>
  )
}

export function NotificationListPage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [filters, setFilters] = useState<NotificationFilters>({ pageNo: 1, pageSize: 10 })
  const [selectedRowKeys, setSelectedRowKeys] = useState<number[]>([])
  const notificationsQuery = useQuery({
    queryKey: ['notifications', filters],
    queryFn: () => notificationApi.listNotifications(filters),
  })
  const unreadQuery = useQuery({
    queryKey: ['unread-count'],
    queryFn: notificationApi.unreadCount,
  })

  const markBatchMutation = useMutation({
    mutationFn: (ids: number[]) => notificationApi.markBatchRead(ids),
    onSuccess: async () => {
      message.success('批量已读成功')
      setSelectedRowKeys([])
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['notifications'] }),
        queryClient.invalidateQueries({ queryKey: ['unread-count'] }),
      ])
    },
  })

  const markReadMutation = useMutation({
    mutationFn: (notificationId: number) => notificationApi.markRead(notificationId),
    onSuccess: async () => {
      message.success('已标记为已读')
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['notifications'] }),
        queryClient.invalidateQueries({ queryKey: ['unread-count'] }),
      ])
    },
  })

  return (
    <div className="page-stack">
      <PageHeaderCard
        title="我的通知"
        description="支持已读/未读筛选、通知类型筛选、精确未读数量展示，以及自动已读和批量已读。"
        extra={<Statistic title="未读数量" value={unreadQuery.data?.unreadCount ?? 0} />}
      />
      <Card variant="borderless" className="data-table-card">
        <div className="filter-bar">
          <Form layout="inline" onFinish={(values) => setFilters({ ...filters, ...values })}>
            <Form.Item label="状态" name="status">
              <Select
                allowClear
                style={{ width: 140 }}
                options={[
                  { label: '未读', value: 'UNREAD' },
                  { label: '已读', value: 'READ' },
                ]}
                size="large"
              />
            </Form.Item>
            <Form.Item label="类型" name="type">
              <Select
                allowClear
                style={{ width: 180 }}
                options={Object.entries(notificationTypeLabelMap).map(([value, label]) => ({ label, value }))}
                size="large"
              />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" size="large">
                筛选
              </Button>
            </Form.Item>
            <Form.Item>
              <Button
                size="large"
                disabled={selectedRowKeys.length === 0}
                onClick={() => markBatchMutation.mutate(selectedRowKeys.map((item) => Number(item)))}
              >
                批量标记已读
              </Button>
            </Form.Item>
          </Form>
        </div>
        <Table<NotificationRecord>
          style={{ marginTop: 16 }}
          rowKey="id"
          dataSource={notificationsQuery.data?.list}
          pagination={false}
          rowSelection={{
            selectedRowKeys,
            onChange: (keys) => setSelectedRowKeys(keys.map((item) => Number(item))),
          }}
          columns={[
            { title: '标题', dataIndex: 'title', width: 320 },
            { 
              title: '摘要', 
              dataIndex: 'summary',
              render: (text: string) => (
                <Typography.Text 
                  style={{ maxWidth: 300, margin: 0, fontSize: 16, fontWeight: 600 }} 
                  ellipsis={{ tooltip: text }}
                >
                  {text}
                </Typography.Text>
              )
            },
            {
              title: '类型',
              dataIndex: 'type',
              width: 140,
              render: (value: NotificationType) => <Tag color="blue">{notificationTypeLabelMap[value]}</Tag>,
            },
            {
              title: '状态',
              dataIndex: 'status',
              width: 100,
              render: (value: 'READ' | 'UNREAD') => <Tag color={value === 'UNREAD' ? 'gold' : 'default'}>{value === 'UNREAD' ? '未读' : '已读'}</Tag>,
            },
            { title: '时间', dataIndex: 'createdAt', width: 180 },
            {
              title: '操作',
              key: 'actions',
              width: 120,
              render: (_, record) => (
                <Space direction="vertical" size="small">
                  <Button type="primary" size="small" onClick={() => navigate(`/notifications/${record.id}`)} style={{ width: '100%' }}>
                    查看详情
                  </Button>
                  <Button
                    size="small"
                    disabled={record.status === 'READ'}
                    loading={markReadMutation.isPending}
                    onClick={() => markReadMutation.mutate(record.id)}
                    style={{ width: '100%' }}
                  >
                    标记已读
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>
    </div>
  )
}

export function NotificationDetailPage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const params = useParams()
  const notificationId = Number(params.notificationId)
  const detailQuery = useQuery({
    queryKey: ['notification-detail', notificationId],
    queryFn: () => notificationApi.getNotification(notificationId),
    enabled: Number.isFinite(notificationId),
  })

  useEffect(() => {
    if (!detailQuery.data) {
      return
    }
    void Promise.all([
      queryClient.invalidateQueries({ queryKey: ['notifications'] }),
      queryClient.invalidateQueries({ queryKey: ['unread-count'] }),
    ])
  }, [detailQuery.data, queryClient])

  if (!Number.isFinite(notificationId)) {
    return <Navigate to="/notifications" replace />
  }

  return (
    <div className="page-stack">
      <PageHeaderCard title="通知详情" description="进入详情后自动标记已读，并保留跳转任务详情的入口。" />
      <div className="settings-container" style={{ maxWidth: 800 }}>
        <Card variant="borderless" className="settings-card" style={{ padding: 24 }}>
          {!detailQuery.data ? (
            <Empty description="未找到通知详情" />
          ) : (
            <Descriptions bordered column={1}>
              <Descriptions.Item label="标题">{detailQuery.data.title}</Descriptions.Item>
              <Descriptions.Item label="通知类型">
                <Tag color="blue">{notificationTypeLabelMap[detailQuery.data.type]}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="摘要">{detailQuery.data.summary}</Descriptions.Item>
              <Descriptions.Item label="详细内容">{detailQuery.data.content}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{detailQuery.data.createdAt}</Descriptions.Item>
              <Descriptions.Item label="跳转目标">
                {detailQuery.data.targetTaskId ? (
                  <Button type="primary" ghost onClick={() => navigate('/tasks')}>前往任务列表查看任务 {detailQuery.data.targetTaskId}</Button>
                ) : (
                  '无'
                )}
              </Descriptions.Item>
            </Descriptions>
          )}
        </Card>
      </div>
    </div>
  )
}

export function UsersPage() {
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState<UserFilters>({ pageNo: 1, pageSize: 10 })
  const [disableOpen, setDisableOpen] = useState(false)
  const [targetUserId, setTargetUserId] = useState<number | null>(null)
  const [disableForm] = Form.useForm()
  const usersQuery = useQuery({
    queryKey: ['admin-users', filters],
    queryFn: () => adminApi.listUsers(filters),
  })

  const enableMutation = useMutation({
    mutationFn: (userId: number) => adminApi.enableUser(userId),
    onSuccess: async () => {
      message.success('用户已启用')
      await queryClient.invalidateQueries({ queryKey: ['admin-users'] })
    },
  })

  const disableMutation = useMutation({
    mutationFn: ({ userId, remark }: { userId: number; remark: string }) => adminApi.disableUser(userId, remark),
    onSuccess: async () => {
      message.success('用户已禁用')
      setDisableOpen(false)
      setTargetUserId(null)
      disableForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['admin-users'] })
    },
  })

  return (
    <div className="page-stack">
      <PageHeaderCard title="用户管理" description="支持按邮箱搜索、查看注册时间，以及执行启用/禁用操作。" />
      <Card variant="borderless">
        <Form layout="inline" onFinish={(values) => setFilters({ ...filters, ...values })}>
          <Form.Item label="邮箱" name="email">
            <Input placeholder="搜索邮箱" />
          </Form.Item>
          <Form.Item label="状态" name="status">
            <Select
              allowClear
              style={{ width: 160 }}
              options={[
                { label: '启用', value: 'ENABLED' },
                { label: '禁用', value: 'DISABLED' },
              ]}
            />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit">
              筛选
            </Button>
          </Form.Item>
        </Form>
        <Table
          style={{ marginTop: 16 }}
          rowKey="id"
          dataSource={usersQuery.data?.list}
          pagination={false}
          columns={[
            { title: '用户 ID', dataIndex: 'id', width: 100 },
            { title: '邮箱', dataIndex: 'email' },
            { title: '角色', dataIndex: 'role', width: 120 },
            {
              title: '状态',
              dataIndex: 'status',
              width: 120,
              render: (value: 'ENABLED' | 'DISABLED') => <Tag color={value === 'ENABLED' ? 'success' : 'error'}>{value}</Tag>,
            },
            { title: '注册时间', dataIndex: 'registeredAt', width: 200 },
            {
              title: '操作',
              key: 'actions',
              width: 220,
              render: (_, record: { id: number; status: 'ENABLED' | 'DISABLED' }) => (
                <Space>
                  <Button
                    size="small"
                    disabled={record.status === 'ENABLED'}
                    onClick={() => enableMutation.mutate(record.id)}
                  >
                    启用
                  </Button>
                  <Button
                    size="small"
                    danger
                    disabled={record.status === 'DISABLED'}
                    onClick={() => {
                      setTargetUserId(record.id)
                      setDisableOpen(true)
                      disableForm.resetFields()
                    }}
                  >
                    禁用
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>
      <Modal
        open={disableOpen}
        title="禁用用户"
        onCancel={() => {
          setDisableOpen(false)
          setTargetUserId(null)
        }}
        onOk={() => disableForm.submit()}
        confirmLoading={disableMutation.isPending}
      >
        <Form
          form={disableForm}
          layout="vertical"
          onFinish={(values: DisableUserRequest) => {
            if (targetUserId === null) {
              return
            }
            disableMutation.mutate({
              userId: targetUserId,
              remark: values.remark,
            })
          }}
        >
          <Form.Item label="禁用原因" name="remark" rules={[{ required: true, message: '请输入禁用原因' }]}>
            <Input.TextArea rows={4} placeholder="请输入禁用备注，例如违规操作、滥用系统等" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export function CategoriesPage() {
  const queryClient = useQueryClient()
  const [form] = Form.useForm()
  const [editing, setEditing] = useState<FileCategory | null>(null)
  const [open, setOpen] = useState(false)
  const categoriesQuery = useQuery({ queryKey: ['file-categories'], queryFn: fileApi.getCategories })

  const saveMutation = useMutation({
    mutationFn: async (values: CreateCategoryRequest & { status?: UpdateCategoryRequest['status'] }) => {
      if (editing) {
        const payload: UpdateCategoryRequest = {
          name: values.name,
          sortNo: values.sortNo,
          status: values.status ?? 'ENABLED',
        }
        return adminApi.updateCategory(editing.id, payload)
      }
      const payload: CreateCategoryRequest = { name: values.name, sortNo: values.sortNo }
      return adminApi.createCategory(payload)
    },
    onSuccess: async () => {
      message.success(editing ? '分类已更新' : '分类已创建')
      setOpen(false)
      setEditing(null)
      form.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['file-categories'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (categoryId: number) => adminApi.deleteCategory(categoryId),
    onSuccess: async () => {
      message.success('分类已删除')
      await queryClient.invalidateQueries({ queryKey: ['file-categories'] })
    },
  })

  return (
    <div className="page-stack">
      <PageHeaderCard
        title="分类管理"
        description="默认分类不可删除，其他分类支持新增、编辑、删除。"
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setEditing(null)
              setOpen(true)
              form.resetFields()
            }}
          >
            新增分类
          </Button>
        }
      />
      <Card variant="borderless">
        <Table
          rowKey="id"
          dataSource={categoriesQuery.data}
          pagination={false}
          columns={[
            { title: '分类 ID', dataIndex: 'id', width: 100 },
            { title: '分类名', dataIndex: 'name' },
            { title: '排序', dataIndex: 'sortNo', width: 100 },
            {
              title: '状态',
              dataIndex: 'status',
              width: 120,
              render: (value: 'ENABLED' | 'DISABLED') => <Tag color={value === 'ENABLED' ? 'success' : 'default'}>{value}</Tag>,
            },
            {
              title: '操作',
              key: 'actions',
              width: 220,
              render: (_, record: FileCategory) => (
                <Space>
                  <Button
                    size="small"
                    disabled={record.isDefault}
                    onClick={() => {
                      setEditing(record)
                      setOpen(true)
                      form.setFieldsValue(record)
                    }}
                  >
                    编辑
                  </Button>
                  <Button
                    size="small"
                    danger
                    icon={<DeleteOutlined />}
                    disabled={record.isDefault}
                    onClick={() => deleteMutation.mutate(record.id)}
                  >
                    删除
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>
      <Modal
        open={open}
        title={editing ? '编辑分类' : '新增分类'}
        onCancel={() => setOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={saveMutation.isPending}
      >
        <Form form={form} layout="vertical" onFinish={(values) => saveMutation.mutate(values)}>
          <Form.Item label="分类名" name="name" rules={[{ required: true }]}>
            <Input placeholder="请输入分类名" />
          </Form.Item>
          <Form.Item label="排序号" name="sortNo" rules={[{ required: true }]}>
            <Input type="number" placeholder="例如 10" />
          </Form.Item>
          {editing ? (
            <Form.Item label="状态" name="status" rules={[{ required: true }]}>
              <Select
                options={[
                  { label: '启用', value: 'ENABLED' },
                  { label: '禁用', value: 'DISABLED' },
                ]}
              />
            </Form.Item>
          ) : null}
        </Form>
      </Modal>
    </div>
  )
}

export function InviteCodesPage() {
  const queryClient = useQueryClient()
  const [filters, setFilters] = useState<InviteCodeFilters>({ pageNo: 1, pageSize: 10 })
  const [singleOpen, setSingleOpen] = useState(false)
  const [batchOpen, setBatchOpen] = useState(false)
  const [remarkOpen, setRemarkOpen] = useState(false)
  const [editingInvite, setEditingInvite] = useState<InviteCodeRecord | null>(null)
  const [singleForm] = Form.useForm()
  const [batchForm] = Form.useForm()
  const [remarkForm] = Form.useForm()
  const inviteCodesQuery = useQuery({
    queryKey: ['invite-codes', filters],
    queryFn: () => adminApi.listInviteCodes(filters),
  })

  const createMutation = useMutation({
    mutationFn: (values: CreateInviteCodeRequest) => adminApi.createInviteCode(values),
    onSuccess: async () => {
      message.success('邀请码已创建')
      setSingleOpen(false)
      singleForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['invite-codes'] })
    },
  })

  const batchMutation = useMutation({
    mutationFn: (values: BatchGenerateInviteCodesRequest) => adminApi.batchGenerateInviteCodes(values),
    onSuccess: async (data) => {
      message.success(`批量生成成功，共 ${data.generateCount} 个，批次号 ${data.batchNo}`)
      setBatchOpen(false)
      batchForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['invite-codes'] })
    },
  })

  const updateRemarkMutation = useMutation({
    mutationFn: ({ inviteCodeId, remark }: { inviteCodeId: number; remark: string }) =>
      adminApi.updateInviteRemark(inviteCodeId, remark),
    onSuccess: async () => {
      message.success('备注已更新')
      setRemarkOpen(false)
      setEditingInvite(null)
      remarkForm.resetFields()
      await queryClient.invalidateQueries({ queryKey: ['invite-codes'] })
    },
  })

  const deleteMutation = useMutation({
    mutationFn: (inviteCodeId: number) => adminApi.deleteInviteCode(inviteCodeId),
    onSuccess: async () => {
      message.success('邀请码已停用并删除')
      await queryClient.invalidateQueries({ queryKey: ['invite-codes'] })
    },
  })

  return (
    <div className="page-stack">
      <PageHeaderCard
        title="邀请码管理"
        description="支持单条创建、批量生成、查看剩余次数、停用删除，并预留导出 xlsx 的入口。"
        extra={
          <Space>
            <Button onClick={() => setBatchOpen(true)}>批量生成</Button>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setSingleOpen(true)}>
              新建邀请码
            </Button>
          </Space>
        }
      />
      <Card variant="borderless">
        <Form layout="inline" onFinish={(values) => setFilters({ ...filters, ...values })}>
          <Form.Item label="关键词" name="keyword">
            <Input placeholder="邀请码 / 备注" />
          </Form.Item>
          <Form.Item label="状态" name="status">
            <Select
              allowClear
              style={{ width: 160 }}
              options={[
                { label: '生效中', value: 'ACTIVE' },
                { label: '已停用', value: 'DISABLED' },
              ]}
            />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit">
              筛选
            </Button>
          </Form.Item>
        </Form>
        <Table
          style={{ marginTop: 16 }}
          rowKey="id"
          dataSource={inviteCodesQuery.data?.list}
          pagination={false}
          columns={[
            { title: '邀请码', dataIndex: 'code', width: 180 },
            { title: '总次数', dataIndex: 'totalQuota', width: 100 },
            { title: '剩余次数', dataIndex: 'remainingQuota', width: 100 },
            { title: '备注', dataIndex: 'remark' },
            { title: '批次号', dataIndex: 'batchNo', width: 180 },
            {
              title: '状态',
              dataIndex: 'status',
              width: 120,
              render: (value: 'ACTIVE' | 'DISABLED') => <Tag color={value === 'ACTIVE' ? 'success' : 'default'}>{value}</Tag>,
            },
            {
              title: '操作',
              key: 'actions',
              width: 220,
              render: (_, record: InviteCodeRecord) => (
                <Space>
                  <Button
                    size="small"
                    onClick={() => {
                      setEditingInvite(record)
                      setRemarkOpen(true)
                      remarkForm.setFieldsValue({ remark: record.remark })
                    }}
                  >
                    修改备注
                  </Button>
                  <Button size="small" danger onClick={() => deleteMutation.mutate(record.id)}>
                    停用删除
                  </Button>
                </Space>
              ),
            },
          ]}
        />
      </Card>
      <Modal
        open={singleOpen}
        title="创建单条邀请码"
        onCancel={() => setSingleOpen(false)}
        onOk={() => singleForm.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Form
          form={singleForm}
          layout="vertical"
          initialValues={{ codeType: 'CUSTOM' }}
          onFinish={(values) => createMutation.mutate(values)}
        >
          <Form.Item label="生成方式" name="codeType" rules={[{ required: true }]}>
            <Select
              options={[
                { label: '自定义', value: 'CUSTOM' },
                { label: '随机生成', value: 'RANDOM' },
              ]}
            />
          </Form.Item>
          <Form.Item shouldUpdate noStyle>
            {({ getFieldValue }) =>
              getFieldValue('codeType') === 'CUSTOM' ? (
                <Form.Item label="邀请码内容" name="code" rules={[{ required: true }]}>
                  <Input placeholder="例如 OS-2026-001" />
                </Form.Item>
              ) : null
            }
          </Form.Item>
          <Form.Item label="可使用次数" name="totalQuota" rules={[{ required: true }]}>
            <Input type="number" placeholder="例如 20" />
          </Form.Item>
          <Form.Item label="备注" name="remark">
            <Input placeholder="可填写课程名或批次说明" />
          </Form.Item>
        </Form>
      </Modal>
      <Modal
        open={batchOpen}
        title="批量生成邀请码"
        onCancel={() => setBatchOpen(false)}
        onOk={() => batchForm.submit()}
        confirmLoading={batchMutation.isPending}
      >
        <Form
          form={batchForm}
          layout="vertical"
          initialValues={{ codeType: 'RANDOM' }}
          onFinish={(values) => batchMutation.mutate(values)}
        >
          <Form.Item label="生成数量" name="generateCount" rules={[{ required: true }]}>
            <Input type="number" placeholder="单次最多 100 个" />
          </Form.Item>
          <Form.Item label="每个邀请码可使用次数" name="totalQuota" rules={[{ required: true }]}>
            <Input type="number" placeholder="例如 10" />
          </Form.Item>
          <Form.Item label="备注" name="remark">
            <Input placeholder="例如 计算机网络期末批次" />
          </Form.Item>
          <Alert type="info" showIcon title="接口文档已预留导出 xlsx 文件流能力，可在联调后补充一键导出按钮。" />
        </Form>
      </Modal>
      <Modal
        open={remarkOpen}
        title="修改邀请码备注"
        onCancel={() => {
          setRemarkOpen(false)
          setEditingInvite(null)
        }}
        onOk={() => remarkForm.submit()}
        confirmLoading={updateRemarkMutation.isPending}
      >
        <Form
          form={remarkForm}
          layout="vertical"
          onFinish={(values: UpdateInviteRemarkRequest) => {
            if (!editingInvite) {
              return
            }
            updateRemarkMutation.mutate({ inviteCodeId: editingInvite.id, remark: values.remark })
          }}
        >
          <Form.Item label="邀请码" required>
            <Input value={editingInvite?.code ?? ''} disabled />
          </Form.Item>
          <Form.Item label="备注" name="remark">
            <Input placeholder="请输入新的备注信息" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export function AdminFilesPage() {
  const [filters, setFilters] = useState<FileFilters>({ pageNo: 1, pageSize: 10 })
  const filesQuery = useQuery({
    queryKey: ['admin-files', filters],
    queryFn: () => adminApi.listAdminFiles(filters),
  })

  return (
    <div className="page-stack">
      <PageHeaderCard title="管理员文件总览" description="支持分页、筛选查看平台文件，并为后续文件删除、任务查看扩展留出入口。" />
      <Card variant="borderless">
        {filesQuery.isError ? (
          <Alert
            type="error"
            showIcon
            message="管理员文件总览加载失败"
            description="当前未再回退到演示文件数据，请检查后端接口。"
            style={{ marginBottom: 16 }}
          />
        ) : null}
        <Form layout="inline" onFinish={(values) => setFilters({ ...filters, ...values })}>
          <Form.Item label="关键词" name="keyword">
            <Input placeholder="文件名关键字" />
          </Form.Item>
          <Form.Item label="可见范围" name="visibility">
            <Select allowClear style={{ width: 160 }} options={visibilityOptions} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit">
              筛选
            </Button>
          </Form.Item>
        </Form>
        <Table
          style={{ marginTop: 16 }}
          rowKey="id"
          dataSource={filesQuery.data?.list}
          pagination={false}
          columns={[
            { title: '文件名', dataIndex: 'sourceFileName' },
            { title: '分类', dataIndex: 'categoryName', width: 120 },
            { title: '上传者', dataIndex: 'uploadUserEmail', width: 180 },
            {
              title: '状态',
              dataIndex: 'generateTotalStatus',
              width: 120,
              render: (value: GenerateStatus) => <StatusTag status={value} />,
            },
            {
              title: '可见范围',
              dataIndex: 'visibility',
              width: 120,
              render: (value: Visibility) => <VisibilityTag visibility={value} />,
            },
            { title: '上传时间', dataIndex: 'uploadTime', width: 180 },
          ]}
        />
      </Card>
    </div>
  )
}
