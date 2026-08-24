#!/usr/bin/env python3
"""i18n locale 文件的语义三方合并（rebase 冲突辅助）。

背景：`web/src/i18n/locales/*.json` 是扁平键值表，魔改线与上游常在同区段
各自增删键，git 文本级合并会产生整块冲突且手工难以对齐。本脚本按键做语义
三方合并：

  - 以 ours（当前分支/upstream 侧）为底，保留其全部键与键序；
  - 应用 theirs（被重放的魔改提交侧）新增的键与修改的值；
  - 双方对同一键做了不同修改时取 theirs（魔改翻译优先）；
  - theirs 相对 base 删除、且 ours 未动的键一并删除。

用法（rebase 停在冲突上时，对每个 locale 文件执行）：

    F=web/src/i18n/locales/zh.json
    git show <commit>^:"$F" > /tmp/base.json     # 被重放提交的父版本
    git show :2:"$F"        > /tmp/ours.json     # 当前分支侧（stage 2）
    git show :3:"$F"        > /tmp/theirs.json   # 魔改提交侧（stage 3）
    python3 scripts/i18n_3way_merge.py /tmp/base.json /tmp/ours.json \
        /tmp/theirs.json "$F"
    git add "$F"

注意：
  - 输出以 ours 键序为准，theirs 新增键追加在尾部；合并完成后建议跑
    `cd web && bun run i18n:sync` 归位键序；
  - 仅适用于「值为字符串」的扁平结构 locale 文件，嵌套 JSON 不适用。
"""

import json
import sys


def load(path):
    with open(path, encoding="utf-8") as f:
        return json.load(f)


def main():
    if len(sys.argv) != 5:
        sys.exit(__doc__)
    base_p, ours_p, theirs_p, out_p = sys.argv[1:5]
    base, ours, theirs = load(base_p), load(ours_p), load(theirs_p)
    result = dict(ours)
    added = changed = removed = 0
    for k, v in theirs.items():
        if k not in base:
            # theirs 新增的键（ours 已有同值则跳过）
            if result.get(k) != v:
                result[k] = v
                added += 1
        elif base[k] != v:
            # theirs 修改了已有键：ours 未动则采纳，双方都动取 theirs
            if k not in result or result[k] != v:
                result[k] = v
                changed += 1
    for k in base:
        # theirs 删除、ours 未恢复的键
        if k not in theirs and k in result and result[k] == base[k]:
            del result[k]
            removed += 1
    with open(out_p, "w", encoding="utf-8") as f:
        json.dump(result, f, ensure_ascii=False, indent=2)
        f.write("\n")
    print(f"{out_p}: +{added} ~{changed} -{removed}")


if __name__ == "__main__":
    main()
