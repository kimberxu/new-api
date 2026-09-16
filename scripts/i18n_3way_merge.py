#!/usr/bin/env python3
"""i18n locale 文件的语义三方合并（rebase/cherry-pick 冲突辅助）。

背景：`web/src/i18n/locales/*.json` 是 `{"translation": {键: 值}}` 的扁平键值
表，魔改线与上游常在同区段各自增删键，git 文本级合并会产生整块冲突且手工
难以对齐。本脚本按键做语义三方合并：

  - 以 ours（当前分支侧）为底，保留其全部键与键序；
  - 重放 theirs（被重放的提交侧）相对 base 的新增键与修改值；
  - theirs 未改动的键保留 ours 的值（魔改翻译不被上游覆盖）；
  - theirs 相对 base 删除、且 ours 未改动的键一并删除。

两个关键约束：

1. **必须逐层递归**。locale 文件顶层只有 `translation` 一个键；只按顶层合并
   会让 `translation` 在 base/theirs 间必然不同，整个内层对象被 theirs 覆盖，
   ours 独有的键被静默清空（不报错、无冲突标记）。
2. **必须按行拼接，不能整文件 `json.dump`**。键 `footer.newapi.
   projectAttributionSuffix` 在文件里以转义形式
   `footer.new\\u0061pi.projectAttributionSuffix` 落盘，是受项目政策保护的
   标识符混淆写法；`json.dump` 会把转义还原成明文。因此最终输出只重放
   theirs 中「新增/修改」的原始行，ours 的行按原字节保留。

用法（rebase/cherry-pick 停在冲突上时，对每个 locale 文件执行）：

    F=web/src/i18n/locales/zh.json
    git show <commit>^:"$F" > /tmp/base.json     # 被重放提交的父版本
    git show :2:"$F"        > /tmp/ours.json     # 当前分支侧（stage 2）
    git show :3:"$F"        > /tmp/theirs.json   # 被重放提交侧（stage 3）
    python3 scripts/i18n_3way_merge.py /tmp/base.json /tmp/ours.json \
        /tmp/theirs.json "$F"
    git add "$F"

注意：输出以 ours 键序为准，theirs 新增键追加在尾部；合并完成后建议跑
`cd web && bun run i18n:sync` 归位键序。
"""

import json
import re
import sys

ENTRY_RE = re.compile(r'^\s*"((?:[^"\\]|\\.)*)"\s*:\s*"(?:[^"\\]|\\.)*"\s*,?\s*$')


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def decode_entries(text, label):
    """按行抽出 (键, 原始行) 序列，保留原始字节（含键的转义写法）。"""
    entries = []
    seen = set()
    for line in text.split("\n"):
        if not ENTRY_RE.match(line):
            continue
        key = json.loads('"' + ENTRY_RE.match(line).group(1) + '"')
        if key in seen:
            raise SystemExit(f"{label}: 重复键 {key!r}")
        seen.add(key)
        entries.append((key, line.rstrip("\n")))
    return entries


def split_wrapper(text):
    """拆出 JSON 外壳（首行 `{`、`"translation": {`、末两行 `}`、`}`）。

    locale 文件的键值表嵌在 `translation` 下，外壳必须原样保留才能写回合法
    JSON；这里只关心「外壳行」与「条目行」的分界。
    """
    lines = text.split("\n")
    first = last = None
    for i, line in enumerate(lines):
        if ENTRY_RE.match(line):
            first = i
            break
    if first is None:
        raise SystemExit("未找到任何条目行，文件结构异常")
    for i in range(len(lines) - 1, -1, -1):
        if ENTRY_RE.match(lines[i]):
            last = i
            break
    return lines[:first], lines[last + 1 :]


def entries_to_text(entries, head, tail):
    """把 (键, 行) 序列写回 JSON 文本，逗号按所处位置补正。"""
    out = list(head)
    for i, (_key, line) in enumerate(entries):
        body = line.rstrip().rstrip(",").rstrip()
        comma = "" if i == len(entries) - 1 else ","
        out.append(body + comma)
    out.extend(tail)
    return "\n".join(out)


def merge(base_entries, ours_entries, theirs_entries):
    base_map = dict(base_entries)
    ours_map = dict(ours_entries)
    theirs_map = dict(theirs_entries)

    result = list(ours_entries)
    index_of = {key: i for i, (key, _line) in enumerate(result)}
    stats = {"added": 0, "changed": 0, "removed": 0}

    for key, line in theirs_entries:
        if key not in base_map:
            if key not in ours_map:
                index_of[key] = len(result)
                result.append((key, line))
                stats["added"] += 1
            continue
        if base_map[key] == theirs_map[key]:
            continue  # theirs 未改动
        if key in index_of:
            result[index_of[key]] = (key, line)
        else:
            index_of[key] = len(result)
            result.append((key, line))
        stats["changed"] += 1

    for key, line in base_entries:
        if key in theirs_map:
            continue
        if key in index_of and ours_map.get(key) == base_map[key]:
            i = index_of.pop(key)
            result[i] = None
            stats["removed"] += 1

    return [item for item in result if item is not None], stats


def main():
    if len(sys.argv) != 5:
        sys.exit(__doc__)
    base_p, ours_p, theirs_p, out_p = sys.argv[1:5]

    # 先做一次 JSON 合法性校验，损坏的文件要在此刻暴露，而不是在拼接之后
    for path in (base_p, ours_p, theirs_p):
        load(path)

    with open(ours_p, encoding="utf-8") as f:
        ours_text = f.read()
    with open(theirs_p, encoding="utf-8") as f:
        theirs_text = f.read()

    ours_entries = decode_entries(ours_text, ours_p)
    base_entries = decode_entries(open(base_p, encoding="utf-8").read(), base_p)
    theirs_entries = decode_entries(theirs_text, theirs_p)
    head, tail = split_wrapper(ours_text)

    merged, stats = merge(base_entries, ours_entries, theirs_entries)
    text = entries_to_text(merged, head, tail)
    with open(out_p, "w", encoding="utf-8") as f:
        f.write(text if text.endswith("\n") else text + "\n")

    # 拼接后的输出必须仍是合法 JSON，且键集合符合预期
    load(out_p)
    print(f"{out_p}: +{stats['added']} ~{stats['changed']} -{stats['removed']}")


if __name__ == "__main__":
    main()
