// 商家后台外壳：左侧导航 + 顶部用户栏 + 右侧内容（Outlet）。最小宽度 1280。
import { Layout, Menu, Avatar, Dropdown, Typography, Space } from 'antd'
import {
  ShopOutlined,
  PlusCircleOutlined,
  ProfileOutlined,
  ThunderboltOutlined,
  LogoutOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../lib/auth'

const { Sider, Header, Content } = Layout

const MENU = [
  { key: '/auctions', icon: <ShopOutlined />, label: '我的拍卖' },
  { key: '/auctions/new', icon: <PlusCircleOutlined />, label: '发布拍卖' },
  { key: '/orders', icon: <ProfileOutlined />, label: '卖家订单' },
  { key: '/demo', icon: <ThunderboltOutlined />, label: '氛围动画 Demo' },
]

export function AppLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const { user, logout } = useAuth()

  // 选中态：取最匹配的前缀。
  const selectedKey =
    MENU.map((m) => m.key)
      .filter((k) => location.pathname === k || location.pathname.startsWith(k + '/'))
      .sort((a, b) => b.length - a.length)[0] || '/auctions'

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider theme="dark" width={220}>
        <div
          style={{
            height: 56,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: '#fff',
            fontWeight: 700,
            fontSize: 16,
            letterSpacing: 1,
          }}
        >
          🔨 竞拍商家后台
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          items={MENU}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Header
          style={{
            background: '#fff',
            display: 'flex',
            justifyContent: 'flex-end',
            alignItems: 'center',
            paddingInline: 24,
            borderBottom: '1px solid #f0f0f0',
          }}
        >
          <Dropdown
            menu={{
              items: [
                { key: 'logout', icon: <LogoutOutlined />, label: '退出登录', onClick: logout },
              ],
            }}
          >
            <Space style={{ cursor: 'pointer' }}>
              <Avatar src={user?.avatar || undefined} icon={<UserOutlined />} />
              <Typography.Text strong>{user?.nickname ?? '未登录'}</Typography.Text>
            </Space>
          </Dropdown>
        </Header>
        <Content style={{ margin: 24 }}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
