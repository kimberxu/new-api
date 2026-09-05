# 同步冲突决策记录模板（`docs/local-github-workflow.md` §冲突决策记录规则）

格式（已有行用 `；` 追加，同 PR 同轮次不另起行）：

    `<短SHA>`：`<YYYY-MM-DD>` 同步 `<N>` 提交；`<拼法一句话>`

## 追加到已有行（照抄拼法，下次同类冲突直接复用）

| 场景 | 拼法 |
|------|------|
| Go 签名变更（上游改函数签名，魔改调用点对齐） | 上游 `<commit>` 把 `<Func>` 改签名 `(<新签名>)`，魔改 `<调用点>` 投递 `<新实参>`（`<nil/省略参数为何安全一句话>`），`go vet` 已过 |
| 上游 BREAKING 删别名/语义（测试语义跟随） | 采纳 `<commit>` BREAKING（`<删了什么>`，仅 `<保留了什么>`）；`<旧断言>` 改 `<新断言>`，`<白名单用例>` 经 `<剥离函数>` 后命中 `<pattern>`；单层 `withTestPolicy`（禁嵌套 `Cleanup` 竞争） |
| locale 整块冲突（上游增删键 + 魔改增删键 + 键序重排三叠加） | `<N>` 批字节保序三方语义（ours 全量为底 + theirs 增删精确重放，不全量 `json.dump` 归一化），`<各批键数一句话>`，增键插 `Zoom` 后，事后 `bun run i18n:sync` 归位；校验口径用 `python` 逐键核对（禁 `grep -E`，行宽截断造假象） |
| JSON 解析归一（AGENTS wrapper 约定） | 取 `<魔改本体>` + `common.UnmarshalJsonStr`（不留 `encoding/json`），`math/rand/v2`（`rand.IntN`） |
| 字段分类 fail-closed（`TestChannelFieldsAreClassified` 红） | `<field>`（`<model 文件>:<行>` `<gorm tag>`，与 `<同类字段>` 同类 `<路由开关/敏感/只读一句话>`）追加到 `<channelXxxFields>` |

## 新建行（仅当确属新增高风险碰撞点）

    | `<文件>` | <魔改内容> | 上游 `<commit>` <改了什么>（`<语义一句话>`） | `<短SHA>`：`<日期>` 同步 `<N>` 提交；<拼法一句话> |

## 收敛规则（防表格稀释：同类冲突照抄决策，不再重新设计）

- `i18n/keys.go` 首次冲突归入 `i18n locale` 行括号备注（同轮次同语义）。
- probe/测试语义跟随归入对应源码行（同提交同 PR 影响面），不另起行。
- 每行竖线数为 5（4 列 + 首尾，表头分隔行亦然），改后 `python` 按行校验 `l.count('|')` 一致即可。
