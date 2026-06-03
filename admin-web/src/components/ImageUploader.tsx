// 图片上传组件：调 POST /api/uploads（B 提供）。受控 value=string[]（url 列表）。
// 前端校验类型 + 大小（见 Task M8）。封面用 max=1，组图 max=9。
import { useState } from 'react'
import { Upload, App } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import type { UploadFile } from 'antd'
import { http } from '../lib/api-client'
import type { ApiEnvelope, UploadData } from '../lib/types'

const ACCEPT = ['image/jpeg', 'image/png', 'image/webp']
const MAX_SIZE = 5 * 1024 * 1024

interface Props {
  value?: string[]
  onChange?: (urls: string[]) => void
  max?: number
}

export function ImageUploader({ value = [], onChange, max = 9 }: Props) {
  const { message } = App.useApp()
  const [uploading, setUploading] = useState(0)

  const fileList: UploadFile[] = value.map((url, i) => ({
    uid: `${i}-${url}`,
    name: `image-${i}`,
    status: 'done',
    url,
  }))

  const beforeUpload = (file: File): boolean => {
    if (!ACCEPT.includes(file.type)) {
      message.error('仅支持 JPEG / PNG / WebP')
      return Upload.LIST_IGNORE as unknown as boolean
    }
    if (file.size > MAX_SIZE) {
      message.error('图片不能超过 5MB')
      return Upload.LIST_IGNORE as unknown as boolean
    }
    return true
  }

  // 自定义上传：multipart → /api/uploads
  const customRequest: NonNullable<React.ComponentProps<typeof Upload>['customRequest']> = async ({
    file,
    onSuccess,
    onError,
  }) => {
    const form = new FormData()
    form.append('file', file as Blob)
    setUploading((n) => n + 1)
    try {
      const resp = await http.post<ApiEnvelope<UploadData>>('/api/uploads', form)
      const url = resp.data.data.url
      onChange?.([...value, url])
      onSuccess?.(resp.data)
    } catch (e) {
      message.error('上传失败，请重试')
      onError?.(e as Error)
    } finally {
      setUploading((n) => n - 1)
    }
  }

  const onRemove = (file: UploadFile) => {
    onChange?.(value.filter((u) => u !== file.url))
  }

  return (
    <Upload
      listType="picture-card"
      fileList={fileList}
      beforeUpload={beforeUpload}
      customRequest={customRequest}
      onRemove={onRemove}
      accept={ACCEPT.join(',')}
      multiple={max > 1}
    >
      {value.length >= max ? null : (
        <div>
          <PlusOutlined />
          <div style={{ marginTop: 8 }}>{uploading > 0 ? '上传中...' : '上传'}</div>
        </div>
      )}
    </Upload>
  )
}
