// 错误码 → 中文映射。严格对应 contract-v2.md §1.2。
// 新增错误码必须先改合同，再在此同步。

export interface ErrorCodeMeta {
  /** 默认中文提示 */
  message: string
  /** 前端处理建议（来自合同「前端建议」列） */
  hint: string
}

export const ERROR_CODES: Record<number, ErrorCodeMeta> = {
  0: { message: '成功', hint: '正常处理' },
  1001: { message: '参数错误', hint: '表单提示' },
  1002: { message: '登录已失效，请重新登录', hint: '跳登录' },
  1003: { message: '无权限执行此操作', hint: '提示无权限' },
  1004: { message: '操作过于频繁，请稍后再试', hint: '禁用按钮后重试' },
  1005: { message: '请求冲突，请刷新后重试', hint: '提示刷新后重试' },
  2001: { message: '拍卖不存在', hint: '返回列表' },
  2002: { message: '拍卖尚未开始', hint: '禁用出价' },
  2003: { message: '拍卖已结束', hint: '展示成交结果' },
  2004: { message: '拍卖已取消', hint: 'toast 提示' },
  2101: { message: '出价低于最低有效价', hint: '预填最低有效价' },
  2102: { message: '出价超过封顶价', hint: '提示封顶价' },
  2103: { message: '出价竞争失败，请刷新后重试', hint: '自动刷新快照，可重试一次' },
  3001: { message: '订单不存在', hint: '返回订单列表' },
  9001: { message: '系统繁忙，请稍后重试', hint: '提示稍后重试' },
  9999: { message: '系统繁忙', hint: 'toast「系统繁忙」' },
}

/** 取错误码对应的中文提示；未知码回退到 9999 文案。 */
export function messageForCode(code: number, fallbackMsg?: string): string {
  if (code in ERROR_CODES) return ERROR_CODES[code].message
  return fallbackMsg || ERROR_CODES[9999].message
}

/** 是否为「需要跳登录」的码。 */
export function isAuthError(code: number): boolean {
  return code === 1002
}
