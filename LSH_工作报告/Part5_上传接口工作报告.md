# Part 5：Role B 上传接口工作报告

> 记录日期：2026-06-03  
> 记录人：LSH / Role B  
> 项目：直播竞拍系统 `auction-system`  
> 工作范围：`POST /api/uploads` 后端上传接口、`/static` 静态访问、移动端 `useUpload` hook

---

## 1. 完成的业务逻辑

本阶段完成的是直播竞拍系统的“商品图片上传”能力。也就是说，已登录用户可以把本地商品图片上传到后端，服务端完成文件大小、图片类型和图片内容校验后，将图片保存到本地上传目录，并返回一个前端可以直接展示的公开 URL。前端则提供可复用的 `useUpload` hook，后续移动端或 PC 商家后台都可以用它完成上传前校验、上传进度展示和失败重试。

已完成业务逻辑：

- 用户可通过 `POST /api/uploads` 上传商品图片。
- 上传请求必须携带 `Authorization: Bearer mock-token-...`，未登录会返回 `401` 和 `code=1002`。
- 上传格式使用 `multipart/form-data`，字段名为 `file`。
- 服务端只接受 `image/jpeg`、`image/png`、`image/webp` 三类图片。
- 服务端限制单文件大小不超过 5MB。
- 服务端不信任客户端文件名和 MIME，而是检查文件 magic number，并通过图片解码读取宽高。
- 服务端将文件保存到 `./uploads/yyyy/mm/dd/`。
- 服务端文件名使用 `sha256 前 16 位 + 短随机 + 扩展名`，避免使用用户上传文件名。
- 服务端返回 `url`、`width`、`height`、`size`。
- 返回的 `url` 是公开访问路径，不暴露 `D:\...`、`/home/...` 等本地绝对路径。
- 后端通过 `/static/...` 暴露上传后的本地文件，MVP 阶段可直接给 `<img>` 使用。
- 前端新增 `useUpload`，上传前先校验类型和大小，上传过程中提供 `progress`，失败后自动重试一次。

简单例子：

卖家在商家后台发布拍卖时选择 `product.jpg`。前端先检查它是不是 JPG/PNG/WebP，且大小不超过 5MB。校验通过后发起：

```text
POST /api/uploads
Authorization: Bearer mock-token-seller-001
Content-Type: multipart/form-data
file=@product.jpg
```

服务端校验图片内容并落盘，例如保存到：

```text
./uploads/2026/06/03/8f14e45fceea167a-a1b2c3d4.jpg
```

返回：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "url": "http://localhost:8080/static/2026/06/03/8f14e45fceea167a-a1b2c3d4.jpg",
    "width": 1080,
    "height": 1080,
    "size": 234567
  }
}
```

前端拿到 `data.url` 后即可回填到拍卖创建或修改表单的 `cover_url` / `images` 字段，也可以直接用于：

```html
<img src="http://localhost:8080/static/2026/06/03/8f14e45fceea167a-a1b2c3d4.jpg" />
```

---

## 2. 工作背景

本阶段对应 Role B 的“场景 5：上传接口（前端 + 后端）”。上传接口是拍卖创建和修改链路的前置能力，因为商家发布拍卖时需要上传商品封面和详情图片。

根据 `docs/contract-v2.md §2.6`、`docs/tasks/backend-agent-tasks.md Task K` 和 `docs/tasks/frontend-agent-tasks.md M8`，本阶段需要满足：

- 后端提供 `POST /api/uploads`。
- 请求为 `multipart/form-data`，字段名 `file`。
- 只接受 `image/jpeg`、`image/png`、`image/webp`。
- 单文件不超过 5MB。
- MVP 可落本地磁盘。
- 前端可通过公开 URL 直接展示图片。
- 响应包含 `url`、`width`、`height`、`size`。
- 前端上传前做本地校验，显示进度，失败重试一次。

本次工作属于 Role B 负责范围，没有实现或修改拍卖创建、出价规则、订单、lifecycle worker，也没有改动 `internal/bid`、`internal/order`、`internal/worker`。

---

## 3. 本次交付结论

本次已完成上传接口的后端实现、静态文件访问、图片安全校验、自动化测试，以及移动端可复用 `useUpload` hook。当前实现已经通过后端全量测试、前端 lint 和前端生产构建验证。

已实现能力：

- `POST /api/uploads`。
- `/static` 静态访问上传目录。
- mock token 鉴权检查。
- 5MB 文件大小限制。
- JPEG / PNG / WebP magic number 校验。
- 图片宽高解析。
- 服务端生成安全文件名。
- 文件落盘到日期目录。
- 返回公开 URL 和图片元信息。
- 前端 `uploadImage` API。
- 前端 `useUpload` hook。
- 上传前本地校验、进度、失败重试一次。
- 后端上传模块单元测试。

---

## 4. 涉及文件

### 4.1 修改文件

- `server-go/main.go`
- `server-go/go.mod`
- `server-go/go.sum`
- `mobile-h5/src/lib/api-client.ts`
- `mobile-h5/src/lib/types.ts`

### 4.2 新增文件

- `server-go/internal/upload/upload.go`
- `server-go/internal/upload/upload_test.go`
- `mobile-h5/src/hooks/useUpload.ts`
- `LSH_工作报告/Part5_上传接口工作报告.md`

---

## 5. 技术实现说明

### 5.1 后端路由注册

`main.go` 中新增：

```go
upload.RegisterRoutes(r, upload.NewHandlerFromEnv())
r.Static("/static", upload.StaticDirFromEnv())
```

效果：

- `POST /api/uploads` 处理上传请求。
- `/static/yyyy/mm/dd/file.ext` 访问本地上传文件。

### 5.2 配置与默认值

上传模块读取环境变量：

```text
UPLOAD_DIR
UPLOAD_PUBLIC_PREFIX
```

默认行为：

- `UPLOAD_DIR` 未配置时使用 `./uploads`。
- `UPLOAD_PUBLIC_PREFIX` 未配置时，根据请求 host 推导 `http://host/static`。
- 如果配置了 `UPLOAD_PUBLIC_PREFIX`，例如未来 CDN 地址，则优先返回配置值拼接后的 URL。

这样可以兼容本地 MVP 和未来 CDN 切换。

### 5.3 鉴权处理

当前后端项目尚未具备完整 auth middleware，因此上传模块做了最小 mock token 检查：

```go
Authorization: Bearer mock-token-seller-001
Authorization: Bearer mock-token-user-001
```

未携带或格式不符合时返回：

```json
{
  "code": 1002,
  "msg": "unauthorized",
  "data": null
}
```

代码中保留生产替换提示：

```go
// TODO(prod): replace with JWT or safer upload auth.
```

### 5.4 文件大小限制

服务端限制：

```go
const MaxFileSize = 5 * 1024 * 1024
```

实现上做了两层保护：

- `http.MaxBytesReader` 限制请求体读取上限。
- `io.LimitReader(file, MaxFileSize+1)` 防止文件实际读取超过 5MB。

超过限制返回：

```json
{
  "code": 1001,
  "msg": "file size must be <= 5MB",
  "data": null
}
```

### 5.5 图片类型与 magic number 校验

服务端不只看客户端传入的 MIME，也不依赖文件扩展名，而是读取文件头：

- JPEG：`FF D8 FF`
- PNG：`89 50 4E 47 0D 0A 1A 0A`
- WebP：`RIFF....WEBP`

随后通过：

```go
image.DecodeConfig(bytes.NewReader(data))
```

解析图片宽高。如果文件头伪装但图片内容无法解码，会被拒绝。

### 5.6 文件名去重策略

文件名由服务端生成：

```text
sha256前16位-短随机.ext
```

例如：

```text
8f14e45fceea167a-a1b2c3d4.jpg
```

实现要点：

- 不使用用户上传的原始文件名。
- 避免路径穿越。
- 使用 `os.O_EXCL` 写入，避免极低概率重名覆盖已有文件。
- 若发生重名，重新生成短随机并重试。

### 5.7 路径安全

上传目录只由服务端配置和日期目录组成：

```text
./uploads/yyyy/mm/dd/
```

服务端通过 `filepath.Abs` 和 `filepath.Rel` 确认最终路径仍在上传根目录下。日期目录由服务端时间生成，不来自用户输入。

### 5.8 公开 URL 生成

返回 URL 通过 public prefix 和相对路径拼接：

```text
{publicPrefix}/yyyy/mm/dd/{filename}
```

不会返回本机绝对路径。

如果没有配置 `UPLOAD_PUBLIC_PREFIX`，请求来自 `localhost:8080` 时会返回：

```text
http://localhost:8080/static/yyyy/mm/dd/file.jpg
```

如果将来切 CDN，只需要配置：

```text
UPLOAD_PUBLIC_PREFIX=https://cdn.example.com/u
```

即可返回：

```text
https://cdn.example.com/u/yyyy/mm/dd/file.jpg
```

### 5.9 前端 API

`mobile-h5/src/lib/api-client.ts` 新增：

```ts
export async function uploadImage(
  file: File,
  onProgress?: (progress: number) => void,
): Promise<UploadResult>
```

实现要点：

- 使用现有 `apiClient`，自动带 `Authorization`、`X-Request-Id`、`X-Client-Type`。
- 使用 `FormData`，字段名为 `file`。
- 使用 axios `onUploadProgress` 计算进度百分比。
- 响应 `code !== 0` 时抛错。

### 5.10 前端 useUpload hook

新增 `useUpload()`：

```ts
const {
  status,
  progress,
  error,
  result,
  upload,
  reset,
  maxUploadSize,
  allowedTypes,
} = useUpload()
```

状态：

```ts
type UploadStatus = 'idle' | 'validating' | 'uploading' | 'success' | 'error'
```

能力：

- 上传前校验 `image/jpeg`、`image/png`、`image/webp`。
- 上传前校验大小 `<= 5MB`。
- 上传中更新 `progress`。
- 上传成功返回 `UploadResult`。
- 上传失败自动重试一次。
- 组件卸载后不继续更新 React state。

---

## 6. 协议或数据流说明

### 6.1 请求协议

```text
POST /api/uploads
Authorization: Bearer mock-token-seller-001
Content-Type: multipart/form-data
```

字段：

```text
file: binary
```

### 6.2 成功响应

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "url": "http://localhost:8080/static/2026/06/03/abc.jpg",
    "width": 1080,
    "height": 1080,
    "size": 234567
  }
}
```

### 6.3 错误响应

未登录：

```json
{
  "code": 1002,
  "msg": "unauthorized",
  "data": null
}
```

文件缺失、非法图片、超大文件：

```json
{
  "code": 1001,
  "msg": "invalid upload file",
  "data": null
}
```

### 6.4 前端调用流程

```text
用户选择图片
  -> useUpload.validateImage(file)
  -> FormData.append('file', file)
  -> POST /api/uploads
  -> onUploadProgress 更新 progress
  -> 成功: status = success, result = UploadResult
  -> 失败: 自动重试一次
  -> 仍失败: status = error, error = message
```

---

## 7. 验收记录

### 7.1 后端自动化测试

执行命令：

```powershell
cd D:\TRAEProj\auction-system\server-go
go test ./internal/upload/...
```

测试结果：

```text
ok   auction-system/server-go/internal/upload
```

执行命令：

```powershell
cd D:\TRAEProj\auction-system\server-go
go test ./...
```

测试结果：

```text
ok   auction-system/server-go
ok   auction-system/server-go/internal/realtime
ok   auction-system/server-go/internal/upload
```

覆盖内容：

- 合法 PNG 上传成功。
- 返回 `code=0`、`url`、`width`、`height`、`size`。
- 返回 URL 不泄露本地绝对路径。
- 未登录返回 401 和 `code=1002`。
- 非真实图片返回 400 和 `code=1001`。
- 超过 5MB 返回 400 和 `code=1001`。

### 7.2 前端自动化验证

执行命令：

```powershell
cd D:\TRAEProj\auction-system\mobile-h5
npm.cmd run lint
```

测试结果：

```text
> mobile-h5@0.0.0 lint
> eslint .
```

执行命令：

```powershell
cd D:\TRAEProj\auction-system\mobile-h5
npm.cmd run build
```

构建结果：

```text
✓ 1807 modules transformed.
✓ built
```

### 7.3 手工验收方式

启动后端：

```powershell
cd D:\TRAEProj\auction-system\server-go
go run .
```

上传图片：

```powershell
curl.exe -X POST http://localhost:8080/api/uploads `
  -H "Authorization: Bearer mock-token-seller-001" `
  -F "file=@../fixtures/product.jpg"
```

预期响应：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "url": "http://localhost:8080/static/2026/06/03/xxxx.jpg",
    "width": 1080,
    "height": 1080,
    "size": 234567
  }
}
```

随后可在浏览器打开 `data.url`，或放入 `<img src="...">` 验证是否可展示。

---

## 8. 当前限制

- 当前鉴权是 mock token 格式校验，不是完整数据库用户鉴权；后续应替换为正式 auth middleware。
- 当前文件落本地磁盘，适合 MVP；生产环境应切换对象存储或 CDN。
- 当前没有实现清理过期上传文件、重复文件引用统计和垃圾回收。
- 当前没有做图片压缩、裁剪、缩略图生成。
- 当前前端只提供 `useUpload` hook，还没有接入具体上传组件 UI。
- 当前没有在 PC 商家后台接入该 hook，因为 `admin-web` 目前尚未建立共享 `src/lib` 结构。
- 当前没有上传进度条组件，仅提供 `progress` 状态供页面组件渲染。

---

## 9. 风险与评审意见

- 本地磁盘上传目录需要加入部署持久化策略，否则服务重启或容器重建可能丢失文件。
- 如果未来多实例部署，单机本地磁盘无法保证所有实例都能访问同一图片，应迁移到对象存储或共享存储。
- `UPLOAD_PUBLIC_PREFIX` 应在生产环境明确配置，避免反向代理或 HTTPS 场景下返回错误 scheme 或 host。
- 当前只检查图片内容和大小，没有做病毒扫描；如果生产开放公网上传，需要增加安全扫描或对象存储安全策略。
- 前端本地校验只能提升体验，服务端校验仍是最终安全边界，当前实现已把类型和大小校验放在服务端。
- URL 形态已经为 CDN 预留，但前端不应 hardcode 域名，只应使用服务端返回的 `url`。

---

## 10. 后续计划

1. 将 `POST /api/uploads` 接入正式 auth middleware，替换 mock token 格式校验。
2. 在 PC 商家后台拍卖发布表单中接入 `useUpload` 或同名上传 hook。
3. 在移动端需要上传的场景中接入 `useUpload`。
4. 补充上传组件 UI，包括选择图片、进度条、错误提示、重试按钮和图片预览。
5. 配置 `.env` 中的 `UPLOAD_DIR` 和 `UPLOAD_PUBLIC_PREFIX`，并在联调环境验证公开 URL。
6. 如进入多实例部署，迁移到对象存储或 CDN，并保持响应字段不变。

---

## 11. 本阶段评审结论

本阶段已完成 Role B 上传接口的前后端基础能力，满足合同中 `POST /api/uploads` 的核心要求：登录、multipart 字段 `file`、图片类型限制、5MB 限制、本地落盘、公开 URL、宽高和大小返回。前端提供了可复用 `useUpload` hook，具备本地校验、进度和失败重试一次能力。

当前实现没有侵入 Role A 的出价、订单和 worker 模块，边界清晰。后端 `go test ./...`、前端 `npm.cmd run lint` 和 `npm.cmd run build` 均已通过，可进入商家发布表单和移动端页面的下一步集成。
