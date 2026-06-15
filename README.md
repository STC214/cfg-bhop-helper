# L4D2 CFG 连跳助手 / L4D2 CFG Bhop Helper

> 不推荐在开启 VAC 的服务器中使用。封号、服务器踢出或其他后果自负。
>
> Do not use this on VAC-enabled servers. You are responsible for any ban, kick,
> or other consequence.

## wait 指令的生与死

这套 CFG 连跳脚本依赖 `wait` 指令。

- 本地单人/本地房主：通常可用。
- 官方匹配服务器：通常不可用，因为官方服务器常见配置会禁用 `wait`。
- 第三方社区服务器：看服务器配置。有些服务器允许 `wait`，有些服务器自带连跳插件，可能根本不需要脚本。

如果服务器禁用了 `wait`，脚本可能只跳一下，或者循环直接断掉。这不是安装失败，而是服务器规则导致。

## Delete 键切换连跳 CFG

助手写入 `autoexec.cfg` 的内容如下。脚本使用标记块包住，方便一键清理。
默认开关键是 Delete 键，在 CFG 中写作 `DEL`。默认状态下空格是普通跳跃，按 `DEL` 后才切换为空格连跳；再次按 `DEL` 会恢复普通跳跃。

```text
// >>> L4D2_AUTO_BHOP_CFG_HELPER_BEGIN
// L4D2 auto bhop cfg helper block.
// It checks wait support before starting the bhop loop.

alias bhop_bind "+jump; wait 2; -jump; wait 2; bhop_jump"
alias bhop_comm "alias bhop_jump bhop_bind"
alias bhop_stop "alias bhop_jump; -jump"
alias +bhop "bhop_comm; bhop_jump"
alias -bhop "bhop_stop"

alias bhop_wait_yes "+bhop"
alias bhop_wait_no "+jump; echo >> L4D2 CFG bhop helper: wait is blocked here, using normal jump."
alias bhop_wait_test "alias bhop_wait_result bhop_wait_yes; wait; bhop_wait_result"
alias wait "alias bhop_wait_result bhop_wait_no; alias wait_result wait_test_fail"
alias +bhop_checked "bhop_wait_test"
alias -bhop_checked "-bhop; -jump"

alias wait_test_pass "echo WAIT_ENABLED"
alias wait_test_fail "echo WAIT_BLOCKED"
alias wait_test "alias wait_result wait_test_pass; wait; wait_result"

alias bhop_on "bind SPACE +bhop_checked; alias toggle_bhop bhop_off; echo >> L4D2 CFG bhop helper: bhop ON.; say_team [BHOP_ON]"
alias bhop_off "bind SPACE +jump; alias toggle_bhop bhop_on; -bhop; -jump; echo >> L4D2 CFG bhop helper: bhop OFF, SPACE is normal jump.; say_team [BHOP_OFF]"
alias bhop_init "bind SPACE +jump; alias toggle_bhop bhop_on; -bhop; -jump"
alias toggle_bhop "bhop_on"

unbind "SPACE"
unbind "SHIFT"
unbind "INS"
unbind "DEL"
bind "SHIFT" "+speed"
bind "SPACE" "+jump"
bind "INS" "wait_test"
bind "DEL" "toggle_bhop"

-jump
bhop_init

echo "-----------------------------------------------------"
echo ">> L4D2 CFG bhop helper loaded: press DEL to toggle bhop."
echo ">> Press INS to test wait support in console."
echo ">> Default state is OFF, so SPACE starts as normal jump."
echo ">> If wait is blocked, bhop mode falls back to normal jump."
echo "-----------------------------------------------------"
// <<< L4D2_AUTO_BHOP_CFG_HELPER_END
```

为了减少编码问题，写入的 CFG 块只使用 ASCII 字符。中文说明留在本文档中。

## 手动安装

1. 找到游戏配置目录：

```text
SteamLibrary\steamapps\common\Left 4 Dead 2\left4dead2\cfg
```

2. 打开或新建：

```text
autoexec.cfg
```

3. 复制上面的 CFG 块到文件末尾。

4. 建议在 Steam 启动选项中加入：

```text
+exec autoexec.cfg
```

## 一键工具

本目录提供一个 Go + Win32 小工具：

```text
cfg-bhop-helper.exe
```

功能：

- 自动扫描常见 Steam / SteamLibrary 安装路径。
- 一键写入 CFG 块，默认使用 Delete 键（`DEL`）切换连跳。
- 默认绑定 Insert 键（`INS`）为 `wait` 检测，按下后在控制台输出 `WAIT_ENABLED` 或 `WAIT_BLOCKED`。
- 可以在界面中自定义连跳开关键，修改后再次点击“一键写入”会覆盖旧设置。
- 提供“键位码”按钮，弹窗显示常用 L4D2 bind 键名。
- 写入前备份现有 `autoexec.cfg`。
- 一键清理 CFG 块，并提示手动移除 Steam 启动项 `+exec autoexec.cfg`。
- 打开游戏 `cfg` 目录。
- exe 文件、标题栏、任务栏和最小化后的系统托盘都使用内置图标。
- 用只读输入框显示 Steam 启动项 `+exec autoexec.cfg`，可选中复制，也可一键复制。

它不会静默修改 Steam 的用户配置文件。Steam 启动项请手动添加，避免误改 Steam 配置。

最小化窗口后，工具会隐藏到右下角系统托盘。点击托盘图标可以恢复窗口。

打包结果是单文件便携版 exe，运行时不需要外置图标或资源文件。

## 构建便携版

```powershell
.\package-portable.ps1
```

输出：

```text
dist/cfg-bhop-helper.exe
```
