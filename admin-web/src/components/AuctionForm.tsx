// 拍卖发布/编辑表单（合同 §2.2）。create 与 edit 共用，一个文件搞定。
// 分三栏：基础信息 / 价格规则 / 时间规则。提交前预览弹窗。
// 注意：合同金额单位为「分」(int64，见 §1)。表单按「元」收集，提交时 ×100 转分。
import { useState } from 'react'
import {
  Form,
  Input,
  InputNumber,
  DatePicker,
  Button,
  Card,
  Row,
  Col,
  Alert,
  Modal,
  Descriptions,
  Space,
  App,
} from 'antd'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import { ImageUploader } from './ImageUploader'
import type { Auction, CreateAuctionRequest } from '../lib/types'

export interface AuctionFormValues {
  title: string
  description?: string
  cover_url?: string[]
  images?: string[]
  stream_url?: string
  start_price: number // 元
  price_step: number // 元
  ceiling_price?: number | null // 元
  start_time: Dayjs
  duration_seconds: number
  extend_seconds: number
  extend_threshold: number
}

interface Props {
  mode: 'create' | 'edit'
  initial?: Auction
  submitting?: boolean
  onSubmit: (payload: CreateAuctionRequest) => Promise<void> | void
}

const yuanToCents = (yuan: number) => Math.round(yuan * 100)
const centsToYuan = (cents: number) => cents / 100
const fmtYuan = (yuan: number | null | undefined) =>
  yuan == null ? '无封顶' : `¥${yuan.toLocaleString('zh-CN', { minimumFractionDigits: 2 })}`

function toPayload(v: AuctionFormValues): CreateAuctionRequest {
  return {
    title: v.title,
    description: v.description || null,
    cover_url: v.cover_url?.[0] || null,
    images: v.images || [],
    stream_url: v.stream_url || null,
    start_price: yuanToCents(v.start_price),
    price_step: yuanToCents(v.price_step),
    ceiling_price: v.ceiling_price == null ? null : yuanToCents(v.ceiling_price),
    start_time: v.start_time.toISOString(),
    duration_seconds: v.duration_seconds,
    extend_seconds: v.extend_seconds,
    extend_threshold: v.extend_threshold,
  }
}

export function AuctionForm({ mode, initial, submitting, onSubmit }: Props) {
  const { message } = App.useApp()
  const [form] = Form.useForm<AuctionFormValues>()
  const [preview, setPreview] = useState<AuctionFormValues | null>(null)

  const startPrice = Form.useWatch('start_price', form)
  const ceiling = Form.useWatch('ceiling_price', form)
  const startTime = Form.useWatch('start_time', form)
  const duration = Form.useWatch('duration_seconds', form)

  const initialValues: Partial<AuctionFormValues> = initial
    ? {
        title: initial.title,
        description: initial.description ?? undefined,
        cover_url: initial.cover_url ? [initial.cover_url] : [],
        images: initial.images ?? [],
        stream_url: initial.stream_url ?? undefined,
        start_price: centsToYuan(initial.start_price),
        price_step: centsToYuan(initial.price_step),
        ceiling_price: initial.ceiling_price == null ? null : centsToYuan(initial.ceiling_price),
        start_time: dayjs(initial.start_time),
        duration_seconds: Math.max(
          30,
          dayjs(initial.original_end_time).diff(dayjs(initial.start_time), 'second'),
        ),
        extend_seconds: initial.extend_seconds,
        extend_threshold: initial.extend_threshold,
      }
    : {
        start_price: 0,
        price_step: 1,
        ceiling_price: null,
        start_time: dayjs().add(10, 'minute'), // Task P2: 默认现在 +10 分钟
        duration_seconds: 600,
        extend_seconds: 30,
        extend_threshold: 30,
        cover_url: [],
        images: [],
      }

  const openPreview = async () => {
    try {
      const v = await form.validateFields()
      setPreview(v)
    } catch {
      message.warning('请先修正表单中的错误')
    }
  }

  const confirm = async () => {
    if (!preview) return
    await onSubmit(toPayload(preview))
    setPreview(null)
  }

  const estEnd = startTime && duration ? dayjs(startTime).add(duration, 'second') : null

  return (
    <>
      <Form form={form} layout="vertical" initialValues={initialValues} requiredMark scrollToFirstError>
        <Row gutter={16} align="stretch">
          {/* 基础信息 */}
          <Col span={8}>
            <Card title="基础信息" style={{ height: '100%' }}>
              <Form.Item
                label="标题"
                name="title"
                rules={[
                  { required: true, message: '请输入标题' },
                  { max: 128, message: '标题不超过 128 字' },
                ]}
              >
                <Input placeholder="例如：景德镇手作青瓷茶杯" showCount maxLength={128} />
              </Form.Item>
              <Form.Item label="描述" name="description" rules={[{ max: 2000, message: '描述不超过 2000 字' }]}>
                <Input.TextArea rows={4} maxLength={2000} showCount placeholder="拍品细节、瑕疵说明等" />
              </Form.Item>
              <Form.Item label="封面图" name="cover_url">
                <ImageUploader max={1} />
              </Form.Item>
              <Form.Item label="组图（≤9）" name="images">
                <ImageUploader max={9} />
              </Form.Item>
              <Form.Item
                label="直播流地址"
                name="stream_url"
                tooltip="可空。留空则详情页展示占位。"
                rules={[{ max: 512, message: '地址过长' }]}
              >
                <Input placeholder="rtmp:// 或 https://（可空）" />
              </Form.Item>
            </Card>
          </Col>

          {/* 价格规则 */}
          <Col span={8}>
            <Card title="价格规则" style={{ height: '100%' }}>
              <Form.Item
                label="起拍价（元）"
                name="start_price"
                rules={[{ required: true, type: 'number', min: 0, message: '起拍价 ≥ 0' }]}
                tooltip="支持 0 元起拍"
              >
                <InputNumber min={0} precision={2} step={1} style={{ width: '100%' }} addonAfter="元" />
              </Form.Item>
              {startPrice === 0 && (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 16 }}
                  message="0 元起拍"
                  description="将以「加价幅度」作为首笔最低有效价。"
                />
              )}
              <Form.Item
                label="加价幅度（元）"
                name="price_step"
                rules={[{ required: true, type: 'number', min: 0.01, message: '加价幅度必须 > 0' }]}
              >
                <InputNumber min={0.01} precision={2} step={1} style={{ width: '100%' }} addonAfter="元" />
              </Form.Item>
              <Form.Item
                label="封顶价（元）"
                name="ceiling_price"
                tooltip="留空 = 无封顶"
                rules={[
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (value == null || value === '') return Promise.resolve()
                      const sp = getFieldValue('start_price') ?? 0
                      if (value <= sp) return Promise.reject(new Error('封顶价需高于起拍价'))
                      return Promise.resolve()
                    },
                  }),
                ]}
              >
                <InputNumber
                  min={0.01}
                  precision={2}
                  step={1}
                  style={{ width: '100%' }}
                  addonAfter="元"
                  placeholder="留空 = 无封顶"
                />
              </Form.Item>
              <Alert
                type="info"
                showIcon
                message={ceiling == null ? '当前：无封顶（价高者得）' : `当前封顶：${fmtYuan(ceiling)}`}
              />
            </Card>
          </Col>

          {/* 时间规则 */}
          <Col span={8}>
            <Card title="时间规则" style={{ height: '100%' }}>
              <Form.Item
                label="开始时间"
                name="start_time"
                rules={[
                  { required: true, message: '请选择开始时间' },
                  {
                    validator: (_, value: Dayjs) =>
                      value && value.isAfter(dayjs())
                        ? Promise.resolve()
                        : Promise.reject(new Error('开始时间必须晚于现在')),
                  },
                ]}
              >
                <DatePicker
                  showTime
                  style={{ width: '100%' }}
                  format="YYYY-MM-DD HH:mm:ss"
                  disabledDate={(d) => d.isBefore(dayjs().startOf('day'))}
                />
              </Form.Item>
              <Form.Item
                label="持续时长（秒）"
                name="duration_seconds"
                rules={[{ required: true, type: 'number', min: 30, max: 86400, message: '30 ~ 86400 秒' }]}
                tooltip="30 秒 ~ 24 小时"
              >
                <InputNumber min={30} max={86400} step={30} style={{ width: '100%' }} addonAfter="秒" />
              </Form.Item>
              {estEnd && (
                <Alert
                  type="info"
                  showIcon
                  style={{ marginBottom: 16 }}
                  message={`预计结束：${estEnd.format('YYYY-MM-DD HH:mm:ss')}`}
                />
              )}
              <Form.Item
                label="延时时长（秒）"
                name="extend_seconds"
                rules={[{ required: true, type: 'number', min: 10, max: 30, message: '10 ~ 30 秒' }]}
                tooltip="临近结束有人出价时，自动延长的秒数"
              >
                <InputNumber min={10} max={30} style={{ width: '100%' }} addonAfter="秒" />
              </Form.Item>
              <Form.Item
                label="延时触发阈值（秒）"
                name="extend_threshold"
                rules={[{ required: true, type: 'number', min: 1, max: 300, message: '1 ~ 300 秒' }]}
                tooltip="剩余时间 ≤ 此值时出价才触发延时"
              >
                <InputNumber min={1} max={300} style={{ width: '100%' }} addonAfter="秒" />
              </Form.Item>
            </Card>
          </Col>
        </Row>

        <div style={{ marginTop: 24, textAlign: 'center' }}>
          <Space>
            <Button onClick={() => form.resetFields()}>重置</Button>
            <Button type="primary" onClick={openPreview} loading={submitting}>
              {mode === 'create' ? '预览并发布' : '预览并保存'}
            </Button>
          </Space>
        </div>
      </Form>

      <Modal
        open={!!preview}
        title="发布预览"
        onOk={confirm}
        onCancel={() => setPreview(null)}
        okText={mode === 'create' ? '确认发布' : '确认保存'}
        confirmLoading={submitting}
        width={640}
      >
        {preview && (
          <Descriptions column={1} bordered size="small">
            <Descriptions.Item label="标题">{preview.title}</Descriptions.Item>
            <Descriptions.Item label="描述">{preview.description || '—'}</Descriptions.Item>
            <Descriptions.Item label="起拍价">{fmtYuan(preview.start_price)}</Descriptions.Item>
            <Descriptions.Item label="加价幅度">{fmtYuan(preview.price_step)}</Descriptions.Item>
            <Descriptions.Item label="封顶价">{fmtYuan(preview.ceiling_price)}</Descriptions.Item>
            <Descriptions.Item label="开始时间">{preview.start_time.format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
            <Descriptions.Item label="持续 / 结束">
              {preview.duration_seconds} 秒 ·{' '}
              {preview.start_time.add(preview.duration_seconds, 'second').format('YYYY-MM-DD HH:mm:ss')}
            </Descriptions.Item>
            <Descriptions.Item label="延时规则">
              剩余 ≤ {preview.extend_threshold}s 出价 → 延 {preview.extend_seconds}s
            </Descriptions.Item>
            <Descriptions.Item label="图片">
              {(preview.images?.length || 0) + (preview.cover_url?.length || 0)} 张
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </>
  )
}
