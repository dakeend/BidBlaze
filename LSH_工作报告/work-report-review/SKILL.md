---
name: work-report-review
description: Create structured Chinese Markdown work reports for completed software development tasks, especially review-ready reports that summarize scope, files changed, implementation details, verification evidence, risks, and next steps. Use when asked to record project work, write a work report, prepare review materials, summarize completed engineering tasks, or add a new PartN report under LSH_工作报告.
---

# Work Report Review

## Purpose

Use this skill to turn completed engineering work into a professional, review-ready Markdown report. The report must be factual, traceable, and useful for later project review.

## Workflow

1. Identify the report scope.
   - Determine the role, scenario, task name, date, and project module.
   - Use a narrow title such as `Part2_events补偿接口工作报告.md`.
   - If writing into `LSH_工作报告`, continue the existing Part number sequence.

2. Gather evidence before writing.
   - Inspect changed files with `git status --short`.
   - Inspect relevant diffs or files with `git diff`, `rg`, and targeted file reads.
   - Collect verification commands and results, such as `go test ./...`, `npm run build`, browser checks, curl output, or WebSocket console output.
   - Separate work performed by Codex from unrelated existing changes.

3. Write with a reviewer's structure.
   Use these sections unless the user requests a different format:
   - 完成的业务逻辑
   - 工作背景
   - 本次交付结论
   - 涉及文件
   - 技术实现说明
   - 协议或数据流说明
   - 验收记录
   - 当前限制
   - 风险与评审意见
   - 后续计划
   - 本阶段评审结论

4. Keep claims tied to evidence.
   - Put completed business logic first, before implementation details.
   - Explain business logic from the user's or system workflow perspective.
   - Add a short concrete example after the business logic list so reviewers can quickly understand the workflow.
   - Mention exact commands that were run.
   - Mention observed outputs in concise form.
   - Mark placeholder or mock behavior clearly.
   - Do not claim production readiness when only a skeleton or mock implementation exists.

5. Preserve ownership boundaries.
   - State which role owns the work.
   - State which modules were not touched when that matters.
   - For this auction project, Role B reports should explicitly note when `internal/bid`, `internal/order`, and `internal/worker` were not changed.

6. End with actionable next steps.
   - List the next technical step.
   - Identify integration points and blockers.
   - Note any contract ambiguity that should be reviewed by the team.

## Report Template

```markdown
# Part N：{{任务名称}}工作报告

> 记录日期：{{YYYY-MM-DD}}
> 记录人：{{姓名 / Role}}
> 项目：{{项目名}}
> 工作范围：{{接口、模块或场景}}

---

## 1. 完成的业务逻辑

{{先用业务/流程语言说明这部分功能让系统具备了什么能力。避免一上来讲文件名或代码结构。}}

已完成业务逻辑：

- {{业务能力 1：例如用户可进入指定拍卖房间并收到房间快照}}
- {{业务能力 2：例如客户端可通过 ping/pong 维持实时连接}}
- {{业务能力 3：例如系统可估算并广播房间在线人数}}

简单例子：

{{用一个很小的真实/准真实流程串起来说明。例如：用户打开 1 号拍卖直播间，浏览器连接 WS，服务端返回 snapshot；用户端发送 ping，服务端返回 pong；房间人数从 0 变为 1，服务端广播 viewer_count。}}

---

## 2. 工作背景

{{说明任务来自哪个角色、哪个场景、为什么要做。}}

---

## 3. 本次交付结论

{{用 1 到 2 段说明是否完成、完成到什么程度、是否通过验收。}}

已实现能力：

- {{能力 1}}
- {{能力 2}}

---

## 4. 涉及文件

### 4.1 修改文件

- `{{path}}`

### 4.2 新增文件

- `{{path}}`

---

## 5. 技术实现说明

### 5.1 {{模块名}}

{{说明职责、边界、关键设计。}}

---

## 6. 协议或数据流说明

{{记录请求、响应、事件、数据流、状态机或关键样例。}}

---

## 7. 验收记录

### 7.1 自动化测试

执行命令：

```powershell
{{command}}
```

测试结果：

```text
{{result}}
```

### 7.2 手工验收

{{记录浏览器、curl、WebSocket、页面操作等验收过程和实际输出。}}

---

## 8. 当前限制

- {{限制 1}}
- {{限制 2}}

---

## 9. 风险与评审意见

- {{风险或评审意见 1}}
- {{风险或评审意见 2}}

---

## 10. 后续计划

1. {{下一步 1}}
2. {{下一步 2}}

---

## 11. 本阶段评审结论

{{给出简洁、客观的评审结论。}}
```

## Style Rules

- Write in Chinese unless the user asks otherwise.
- Use precise engineering language, not marketing language.
- Prefer concrete nouns: interface, file, module, command, output, limitation.
- Put “完成的业务逻辑” as section 1 so reviewers see the functional value first.
- Include one simple example in “完成的业务逻辑” to connect the bullet list into a readable business flow.
- Keep sections numbered for easy review.
- Avoid exaggeration. Use “骨架”“占位”“待接入” when implementation is not complete.
- Use Markdown code fences for commands, outputs, and JSON samples.
- Do not include unrelated git changes in the report.
