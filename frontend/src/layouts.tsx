import {
  BellOutlined,
  FileOutlined,
  FolderOpenOutlined,
  LockOutlined,
  LogoutOutlined,
  NotificationOutlined,
  TeamOutlined,
  UploadOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { useQuery } from '@tanstack/react-query'
import { Avatar, Badge, Button, Dropdown, Layout, Menu, Space, Typography } from 'antd'
import type { MenuProps } from 'antd'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'

import { notificationApi } from './api.ts'
import { useAuth } from './auth.tsx'
import { APP_NAME } from './config.ts'

const { Header, Content, Sider } = Layout

function buildMenuItems(isAdmin: boolean): MenuProps['items'] {
  const baseItems: MenuProps['items'] = [
    { key: '/files', icon: <FileOutlined />, label: '文件列表' },
    { key: '/upload', icon: <UploadOutlined />, label: '上传文件' },
    { key: '/tasks', icon: <FolderOpenOutlined />, label: '我的任务' },
    { key: '/notifications', icon: <NotificationOutlined />, label: '我的通知' },
    { key: '/password/change', icon: <LockOutlined />, label: '修改密码' },
  ]

  if (!isAdmin) {
    return baseItems.filter((item) => item?.key !== '/upload' && item?.key !== '/tasks')
  }

  return [
    ...baseItems,
    { type: 'divider' },
    { key: '/admin/users', icon: <TeamOutlined />, label: '用户管理' },
    { key: '/admin/categories', icon: <FolderOpenOutlined />, label: '分类管理' },
    { key: '/admin/invite-codes', icon: <BellOutlined />, label: '邀请码管理' },
    { key: '/admin/files', icon: <FileOutlined />, label: '文件总览' },
  ]
}

export function AuthLayout() {
  return (
    <div className="auth-shell">
      <div className="auth-shell__hero">
        <Typography.Title level={2}>{APP_NAME}</Typography.Title>
        <Typography.Paragraph>
          把课件、讲义、笔记转成可预览、可练习、可下载的期末复习系统。
        </Typography.Paragraph>
      </div>
      <div className="auth-shell__panel">
        <Outlet />
      </div>
    </div>
  )
}

export function AppLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, isAdmin, logout } = useAuth()

  const unreadQuery = useQuery({
    queryKey: ['unread-count'],
    queryFn: notificationApi.unreadCount,
  })

  const selectedKey = location.pathname.startsWith('/files/')
    ? '/files'
    : location.pathname.startsWith('/notifications/')
      ? '/notifications'
      : location.pathname

  const menuItems: MenuProps['items'] = [
    {
      key: 'logout',
      icon: <LogoutOutlined />,
      label: '退出登录',
      onClick: () => {
        void logout().then(() => navigate('/login', { replace: true }))
      },
    },
  ]

  return (
    <Layout className="app-shell">
      <Sider
        breakpoint="lg"
        collapsedWidth={0}
        width={240}
        theme="light"
        className="app-shell__sider"
      >
        <div className="app-shell__brand">
          <Typography.Title level={4}>{APP_NAME}</Typography.Title>
          <Typography.Text type="secondary">期末复习生成与学习平台</Typography.Text>
        </div>
        <Menu
          mode="inline"
          selectedKeys={[selectedKey]}
          items={buildMenuItems(isAdmin)}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header className="app-shell__header">
          <Space size="middle">
            <Badge count={unreadQuery.data?.unreadCount ?? 0} overflowCount={99}>
              <Button
                type="text"
                icon={<NotificationOutlined />}
                onClick={() => navigate('/notifications')}
              >
                通知
              </Button>
            </Badge>
            <Dropdown menu={{ items: menuItems }} trigger={['click']}>
              <Button type="text">
                <Space>
                  <Avatar size="small" icon={<UserOutlined />} />
                  <span>{user?.email ?? '未登录用户'}</span>
                </Space>
              </Button>
            </Dropdown>
          </Space>
        </Header>
        <Content className="app-shell__content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
