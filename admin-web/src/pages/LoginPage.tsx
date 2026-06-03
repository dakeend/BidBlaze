// 登录页（合同 §2.1，mock 登录）。极简：仅 nickname。PC 后台仅放卖家。
import { useState } from 'react'
import { Card, Form, Input, Button, Typography, Alert, App } from 'antd'
import { UserOutlined } from '@ant-design/icons'
import { useLocation, useNavigate } from 'react-router-dom'
import { useAuth, roleFromToken } from '../lib/auth'
import { ApiError } from '../lib/api-client'

const { Title, Paragraph, Text } = Typography

export function LoginPage() {
  const { login, logout } = useAuth()
  const { message } = App.useApp()
  const navigate = useNavigate()
  const location = useLocation()
  const [loading, setLoading] = useState(false)
  const from = (location.state as { from?: string } | null)?.from || '/auctions'

  const onFinish = async ({ nickname }: { nickname: string }) => {
    setLoading(true)
    try {
      const data = await login(nickname.trim())
      if (roleFromToken(data.token) !== 'seller') {
        logout()
        message.error('PC 商家后台仅限卖家。卖家昵称请以「主播 / 商家 / 卖家」开头。')
        return
      }
      message.success(`欢迎，${data.user.nickname}`)
      navigate(from, { replace: true })
    } catch (e) {
      const msg = e instanceof ApiError ? e.message : '登录失败，请重试'
      message.error(msg)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'grid',
        placeItems: 'center',
        background: 'linear-gradient(135deg,#1e1b4b,#4c1d95)',
      }}
    >
      <Card style={{ width: 400 }} variant="borderless">
        <Title level={3} style={{ textAlign: 'center', marginBottom: 4 }}>
          🔨 竞拍商家后台
        </Title>
        <Paragraph type="secondary" style={{ textAlign: 'center' }}>
          输入昵称即可登录（mock 登录）
        </Paragraph>
        <Form layout="vertical" onFinish={onFinish} requiredMark={false}>
          <Form.Item
            name="nickname"
            label="昵称"
            rules={[
              { required: true, message: '请输入昵称' },
              { max: 32, message: '昵称不超过 32 字' },
            ]}
          >
            <Input
              prefix={<UserOutlined />}
              placeholder="例如：主播阿明"
              size="large"
              allowClear
              autoFocus
            />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" size="large" block loading={loading}>
              登录
            </Button>
          </Form.Item>
        </Form>
        <Alert
          type="info"
          showIcon
          message={
            <Text style={{ fontSize: 12 }}>
              卖家昵称以「主播 / 商家 / 卖家」开头；其它视为买家（买家请走移动端）。
            </Text>
          }
        />
      </Card>
    </div>
  )
}
