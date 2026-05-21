import { RouterProvider, createBrowserRouter } from 'react-router-dom'

import { GuestOnlyRoute, RequireAdmin, RequireAuth } from './auth.tsx'
import { AppLayout, AuthLayout } from './layouts.tsx'
import {
  AdminFilesPage,
  CategoriesPage,
  ChangePasswordPage,
  FileDetailPage,
  FileListPage,
  ForgotPasswordPage,
  GeneratedHtmlPreviewPage,
  InviteCodesPage,
  LoginPage,
  NotificationDetailPage,
  NotificationListPage,
  RegisterPage,
  TaskListPage,
  UploadPage,
  UsersPage,
} from './pages.tsx'

const router = createBrowserRouter([
  {
    element: <GuestOnlyRoute />,
    children: [
      {
        element: <AuthLayout />,
        children: [
          { path: '/login', element: <LoginPage /> },
          { path: '/register', element: <RegisterPage /> },
          { path: '/forgot-password', element: <ForgotPasswordPage /> },
        ],
      },
    ],
  },
  {
    element: <RequireAuth />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { index: true, element: <FileListPage /> },
          { path: '/files', element: <FileListPage /> },
          { path: '/files/:fileId', element: <FileDetailPage /> },
          { path: '/files/:fileId/results/:itemType', element: <GeneratedHtmlPreviewPage /> },
          { path: '/tasks', element: <TaskListPage /> },
          { path: '/password/change', element: <ChangePasswordPage /> },
          {
            element: <RequireAdmin />,
            children: [
              { path: '/upload', element: <UploadPage /> },
              { path: '/notifications', element: <NotificationListPage /> },
              { path: '/notifications/:notificationId', element: <NotificationDetailPage /> },
              { path: '/admin/users', element: <UsersPage /> },
              { path: '/admin/categories', element: <CategoriesPage /> },
              { path: '/admin/invite-codes', element: <InviteCodesPage /> },
              { path: '/admin/files', element: <AdminFilesPage /> },
            ],
          },
        ],
      },
    ],
  },
])

function App() {
  return <RouterProvider router={router} />
}

export default App
