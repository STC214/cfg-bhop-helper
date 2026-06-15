package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	appTitle = "L4D2 CFG Bhop Helper"

	wmAppStatus = 0x8001
	wmAppPath   = 0x8002
	wmTrayIcon  = 0x8003

	idPathEdit   = 101
	idDetectBtn  = 102
	idInstallBtn = 103
	idCleanBtn   = 104
	idOpenBtn    = 105
	idStatus     = 106
	idLaunchEdit = 107
	idCopyBtn    = 108
	idToggleEdit = 109
	idKeyHelpBtn = 110

	WM_CREATE         = 0x0001
	WM_CLOSE          = 0x0010
	WM_DESTROY        = 0x0002
	WM_COMMAND        = 0x0111
	WM_CTLCOLORSTATIC = 0x0138
	WM_CTLCOLOREDIT   = 0x0133
	WM_SETFONT        = 0x0030
	WM_GETTEXT        = 0x000D
	WM_GETTEXTLENGTH  = 0x000E
	WM_SYSCOMMAND     = 0x0112
	WM_LBUTTONUP      = 0x0202
	WM_RBUTTONUP      = 0x0205
	WM_LBUTTONDBLCLK  = 0x0203
	BN_CLICKED        = 0
	SC_MINIMIZE       = 0xF020
	WS_OVERLAPPED     = 0x00000000
	WS_CAPTION        = 0x00C00000
	WS_SYSMENU        = 0x00080000
	WS_MINIMIZEBOX    = 0x00020000
	WS_VISIBLE        = 0x10000000
	WS_CHILD          = 0x40000000
	WS_TABSTOP        = 0x00010000
	WS_BORDER         = 0x00800000
	ES_AUTOHSCROLL    = 0x0080
	ES_READONLY       = 0x0800
	BS_PUSHBUTTON     = 0x00000000
	SS_LEFT           = 0x00000000
	WM_SETICON        = 0x0080
	IMAGE_ICON        = 1
	ICON_SMALL        = 0
	ICON_BIG          = 1
	LR_DEFAULTCOLOR   = 0x0000
	SW_SHOW           = 5
	SW_HIDE           = 0
	SW_RESTORE        = 9
	NIM_ADD           = 0x00000000
	NIM_DELETE        = 0x00000002
	NIF_MESSAGE       = 0x00000001
	NIF_ICON          = 0x00000002
	NIF_TIP           = 0x00000004
	MB_OK             = 0x00000000
)

const cfgBlockTemplate = `// >>> L4D2_AUTO_BHOP_CFG_HELPER_BEGIN
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
{{DEFAULT_TOGGLE_UNBIND}}unbind "{{TOGGLE_KEY}}"
bind "SHIFT" "+speed"
bind "SPACE" "+jump"
bind "INS" "wait_test"
bind "{{TOGGLE_KEY}}" "toggle_bhop"

-jump
bhop_init

echo "-----------------------------------------------------"
echo ">> L4D2 CFG bhop helper loaded: press {{TOGGLE_KEY}} to toggle bhop."
echo ">> Press INS to test wait support in console."
echo ">> Default state is OFF, so SPACE starts as normal jump."
echo ">> If wait is blocked, bhop mode falls back to normal jump."
echo "-----------------------------------------------------"
// <<< L4D2_AUTO_BHOP_CFG_HELPER_END
`

const (
	blockBegin       = "// >>> L4D2_AUTO_BHOP_CFG_HELPER_BEGIN"
	blockEnd         = "// <<< L4D2_AUTO_BHOP_CFG_HELPER_END"
	defaultToggleKey = "DEL"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")

	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
	procOpenClipboard       = user32.NewProc("OpenClipboard")
	procEmptyClipboard      = user32.NewProc("EmptyClipboard")
	procSetClipboardData    = user32.NewProc("SetClipboardData")
	procCloseClipboard      = user32.NewProc("CloseClipboard")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procLoadImageW          = user32.NewProc("LoadImageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procSetWindowTextW      = user32.NewProc("SetWindowTextW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procSetBkMode           = gdi32.NewProc("SetBkMode")
	procSetTextColor        = gdi32.NewProc("SetTextColor")
	procCreateSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	procCreateFontW         = gdi32.NewProc("CreateFontW")
	procGetModuleHandleW    = kernel32.NewProc("GetModuleHandleW")
	procGetLogicalDrives    = kernel32.NewProc("GetLogicalDrives")
	procGlobalAlloc         = kernel32.NewProc("GlobalAlloc")
	procGlobalFree          = kernel32.NewProc("GlobalFree")
	procGlobalLock          = kernel32.NewProc("GlobalLock")
	procGlobalUnlock        = kernel32.NewProc("GlobalUnlock")
	procLstrcpyW            = kernel32.NewProc("lstrcpyW")
	procShellExecuteW       = shell32.NewProc("ShellExecuteW")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
)

const (
	launchOption   = "+exec autoexec.cfg"
	CF_UNICODETEXT = 13
	GMEM_MOVEABLE  = 0x0002
)

type point struct{ x, y int32 }
type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}
type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}
type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}
type notifyIconData struct {
	cbSize           uint32
	hwnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         guid
	hBalloonIcon     uintptr
}

var (
	controls    = map[int]uintptr{}
	font        uintptr
	bgBrush     uintptr
	iconBig     uintptr
	iconSmall   uintptr
	mainHwnd    uintptr
	trayVisible bool
	uiMu        sync.Mutex
	opMu        sync.Mutex
	opRunning   bool
	pendingPath string
	pendingText string
)

func main() {
	runtime.LockOSThread()

	hInstance, _, _ := procGetModuleHandleW.Call(0)
	className := utf16Ptr("L4D2CfgBhopHelper")
	cursor, _, _ := procLoadCursorW.Call(0, uintptr(32512))
	iconBig = loadAppIcon(hInstance, 256)
	iconSmall = loadAppIcon(hInstance, 16)
	bgBrush, _, _ = procCreateSolidBrush.Call(0x202020)

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   syscall.NewCallback(wndProc),
		hInstance:     hInstance,
		hIcon:         iconBig,
		hCursor:       cursor,
		hbrBackground: bgBrush,
		lpszClassName: className,
		hIconSm:       iconSmall,
	}
	procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(utf16Ptr(appTitle))),
		WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU|WS_MINIMIZEBOX|WS_VISIBLE,
		260, 160, 720, 500,
		0, 0, hInstance, 0,
	)
	mainHwnd = hwnd
	applyWindowIcons(hwnd)
	procShowWindow.Call(hwnd, SW_SHOW)

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func wndProc(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	switch message {
	case WM_CREATE:
		mainHwnd = hwnd
		createUI(hwnd)
		startOperation("正在后台查找 cfg 目录，界面可继续操作。", detectPathAsync)
		return 0
	case WM_COMMAND:
		id := int(wParam & 0xffff)
		code := int((wParam >> 16) & 0xffff)
		if code == BN_CLICKED {
			switch id {
			case idDetectBtn:
				startOperation("正在后台查找 cfg 目录...", detectPathAsync)
			case idInstallBtn:
				cfgDir := strings.TrimSpace(getControlText(controls[idPathEdit]))
				toggleKey := getControlText(controls[idToggleEdit])
				startOperation("正在后台写入 autoexec.cfg...", func() { installCFGAsync(cfgDir, toggleKey) })
			case idCleanBtn:
				cfgDir := strings.TrimSpace(getControlText(controls[idPathEdit]))
				startOperation("正在后台清理 autoexec.cfg...", func() { cleanCFGAsync(cfgDir) })
			case idOpenBtn:
				cfgDir := strings.TrimSpace(getControlText(controls[idPathEdit]))
				startOperation("正在打开 cfg 目录...", func() { openCfgDirAsync(cfgDir) })
			case idCopyBtn:
				if copyToClipboard(launchOption) {
					setStatus("已复制 Steam 启动项。")
				} else {
					setStatus("复制失败：无法打开剪贴板。")
				}
			case idKeyHelpBtn:
				showKeyHelp()
			}
		}
		return 0
	case wmAppStatus:
		setStatus(takePendingStatus())
		return 0
	case wmAppPath:
		path, text := takePendingPath()
		if path != "" {
			setWindowText(controls[idPathEdit], path)
		}
		setStatus(text)
		return 0
	case wmTrayIcon:
		switch uint32(lParam) {
		case WM_LBUTTONUP, WM_LBUTTONDBLCLK, WM_RBUTTONUP:
			restoreFromTray()
		}
		return 0
	case WM_SYSCOMMAND:
		if wParam&0xfff0 == SC_MINIMIZE {
			minimizeToTray()
			return 0
		}
	case WM_CTLCOLORSTATIC, WM_CTLCOLOREDIT:
		hdc := wParam
		procSetBkMode.Call(hdc, 1)
		procSetTextColor.Call(hdc, 0xF2F2F2)
		return bgBrush
	case WM_CLOSE:
		deleteTrayIcon()
		procDestroyWindow.Call(hwnd)
		return 0
	case WM_DESTROY:
		deleteTrayIcon()
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}

func createUI(hwnd uintptr) {
	font, _, _ = procCreateFontW.Call(neg(16), 0, 0, 0, 500, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(utf16Ptr("Segoe UI"))))

	createText(hwnd, 24, 20, 660, 22, "L4D2 CFG 连跳助手：写入 autoexec.cfg，清理时只删除本工具写入的标记块。")
	createText(hwnd, 24, 54, 660, 22, "提示：不推荐在 VAC 服务器中使用。CFG wait 连跳依赖服务器允许 wait 指令。")
	createText(hwnd, 24, 92, 80, 22, "cfg 路径")
	controls[idPathEdit] = createControl("EDIT", "", WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 104, 88, 470, 28, hwnd, idPathEdit)
	controls[idDetectBtn] = createControl("BUTTON", "自动查找", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 588, 88, 96, 30, hwnd, idDetectBtn)
	controls[idInstallBtn] = createControl("BUTTON", "一键写入", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 104, 142, 120, 36, hwnd, idInstallBtn)
	controls[idCleanBtn] = createControl("BUTTON", "一键清理", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 244, 142, 120, 36, hwnd, idCleanBtn)
	controls[idOpenBtn] = createControl("BUTTON", "打开目录", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 384, 142, 120, 36, hwnd, idOpenBtn)
	createText(hwnd, 24, 204, 130, 22, "连跳开关键")
	controls[idToggleEdit] = createControl("EDIT", defaultToggleKey, WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL, 154, 200, 120, 28, hwnd, idToggleEdit)
	controls[idKeyHelpBtn] = createControl("BUTTON", "键位码", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 294, 199, 90, 30, hwnd, idKeyHelpBtn)
	createText(hwnd, 404, 204, 270, 22, "默认 Delete：DEL。修改后点“一键写入”覆盖设置。")
	createText(hwnd, 24, 242, 660, 22, "wait 检测键：INS。按下后在游戏控制台查看 WAIT_ENABLED / WAIT_BLOCKED。")
	createText(hwnd, 24, 292, 130, 22, "Steam 启动项")
	controls[idLaunchEdit] = createControl("EDIT", launchOption, WS_CHILD|WS_VISIBLE|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL|ES_READONLY, 154, 288, 350, 28, hwnd, idLaunchEdit)
	controls[idCopyBtn] = createControl("BUTTON", "复制启动项", WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_PUSHBUTTON, 524, 287, 120, 30, hwnd, idCopyBtn)
	controls[idStatus] = createControl("STATIC", "", WS_CHILD|WS_VISIBLE|SS_LEFT, 24, 340, 660, 110, hwnd, idStatus)
}

func loadAppIcon(hInstance uintptr, size int32) uintptr {
	icon, _, _ := procLoadImageW.Call(
		hInstance,
		1,
		IMAGE_ICON,
		uintptr(size),
		uintptr(size),
		LR_DEFAULTCOLOR,
	)
	return icon
}

func applyWindowIcons(hwnd uintptr) {
	if iconBig != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_BIG, iconBig)
	}
	if iconSmall != 0 {
		procSendMessageW.Call(hwnd, WM_SETICON, ICON_SMALL, iconSmall)
	}
}

func minimizeToTray() {
	if mainHwnd == 0 {
		return
	}
	if addTrayIcon() {
		procShowWindow.Call(mainHwnd, SW_HIDE)
	}
}

func restoreFromTray() {
	if mainHwnd == 0 {
		return
	}
	deleteTrayIcon()
	procShowWindow.Call(mainHwnd, SW_RESTORE)
	procSetForegroundWindow.Call(mainHwnd)
}

func addTrayIcon() bool {
	if trayVisible || mainHwnd == 0 {
		return trayVisible
	}
	nid := newTrayIconData()
	ok, _, _ := procShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	trayVisible = ok != 0
	return trayVisible
}

func deleteTrayIcon() bool {
	if !trayVisible || mainHwnd == 0 {
		return !trayVisible
	}
	nid := newTrayIconData()
	ok, _, _ := procShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&nid)))
	if ok != 0 {
		trayVisible = false
	}
	return !trayVisible
}

func newTrayIconData() notifyIconData {
	var nid notifyIconData
	nid.cbSize = uint32(unsafe.Sizeof(nid))
	nid.hwnd = mainHwnd
	nid.uID = 1
	nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP
	nid.uCallbackMessage = wmTrayIcon
	nid.hIcon = iconSmall
	if nid.hIcon == 0 {
		nid.hIcon = iconBig
	}
	copyUTF16(nid.szTip[:], appTitle)
	return nid
}

func createText(parent uintptr, x, y, w, h int32, text string) uintptr {
	return createControl("STATIC", text, WS_CHILD|WS_VISIBLE|SS_LEFT, x, y, w, h, parent, 0)
}

func createControl(class, text string, style uintptr, x, y, w, h int32, parent uintptr, id int) uintptr {
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr(class))),
		uintptr(unsafe.Pointer(utf16Ptr(text))),
		style,
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		parent, uintptr(id), 0, 0,
	)
	if font != 0 {
		procSendMessageW.Call(hwnd, WM_SETFONT, font, 1)
	}
	return hwnd
}

func showKeyHelp() {
	text := strings.Join([]string{
		"常用 L4D2 bind 键位码",
		"",
		"Delete / Insert: DEL, INS",
		"Home / End: HOME, END",
		"Page Up / Page Down: PGUP, PGDN",
		"方向键: UPARROW, DOWNARROW, LEFTARROW, RIGHTARROW",
		"功能键: F1 - F12",
		"小键盘: KP_0 - KP_9",
		"小键盘符号: KP_SLASH, KP_MULTIPLY, KP_MINUS, KP_PLUS, KP_ENTER, KP_DEL",
		"鼠标: MOUSE1 - MOUSE5, MWHEELUP, MWHEELDOWN",
		"常规键: A - Z, 0 - 9, SPACE, SHIFT, CTRL, ALT, TAB, ESC",
		"",
		"INS 已固定为 wait 检测键，按下后在控制台查看 WAIT_ENABLED / WAIT_BLOCKED。",
		"本工具不允许把开关键设为 SPACE，因为空格需要保留为跳跃键。",
		"本工具也不允许把开关键设为 INS，因为 INS 已保留为 wait 检测键。",
		"本工具也不允许把开关键设为 SHIFT，因为 SHIFT 会被恢复为 +speed。",
		"修改开关键后，点击“一键写入”会覆盖本工具的 CFG 标记块。",
	}, "\r\n")
	procMessageBoxW.Call(mainHwnd, uintptr(unsafe.Pointer(utf16Ptr(text))), uintptr(unsafe.Pointer(utf16Ptr("键位码参考"))), MB_OK)
}

func startOperation(startText string, fn func()) {
	opMu.Lock()
	if opRunning {
		opMu.Unlock()
		setStatus("已有后台操作正在执行，请稍候。")
		return
	}
	opRunning = true
	opMu.Unlock()

	setStatus(startText)
	go func() {
		defer finishOperation()
		fn()
	}()
}

func finishOperation() {
	opMu.Lock()
	opRunning = false
	opMu.Unlock()
}

func detectPathAsync() {
	if cfg := firstDetectedCfgDir(); cfg != "" {
		postPath(cfg, "已找到 cfg 目录。点击“一键写入”会备份并更新 autoexec.cfg。")
		return
	}
	postStatus("未找到 cfg 目录。请手动填入 SteamLibrary\\steamapps\\common\\Left 4 Dead 2\\left4dead2\\cfg。")
}

func installCFGAsync(cfgDir, toggleKeyInput string) {
	if !isCfgDir(cfgDir) {
		postStatus("路径无效：请选择 Left 4 Dead 2\\left4dead2\\cfg 目录。")
		return
	}
	toggleKey, err := normalizeToggleKey(toggleKeyInput)
	if err != nil {
		postStatus("开关键无效：" + err.Error())
		return
	}
	autoexec := filepath.Join(cfgDir, "autoexec.cfg")
	original, _ := os.ReadFile(autoexec)
	if len(original) > 0 {
		backup := filepath.Join(cfgDir, fmt.Sprintf("autoexec.cfg.bak.%s", time.Now().Format("20060102-150405")))
		if err := os.WriteFile(backup, original, 0644); err != nil {
			postStatus("备份失败：" + err.Error())
			return
		}
	}
	next := removeBlock(string(original))
	if strings.TrimSpace(next) != "" && !strings.HasSuffix(next, "\n") {
		next += "\r\n"
	}
	next += "\r\n" + buildCFGBlock(toggleKey)
	if err := os.WriteFile(autoexec, []byte(next), 0644); err != nil {
		postStatus("写入失败：" + err.Error())
		return
	}
	postStatus("写入完成：autoexec.cfg 已更新。连跳默认关闭，按 " + toggleKey + " 切换。Steam 启动项建议添加：+exec autoexec.cfg")
}

func buildCFGBlock(toggleKey string) string {
	defaultUnbind := ""
	if toggleKey != defaultToggleKey {
		defaultUnbind = "unbind \"" + defaultToggleKey + "\"\n"
	}
	block := strings.ReplaceAll(cfgBlockTemplate, "{{DEFAULT_TOGGLE_UNBIND}}", defaultUnbind)
	return strings.ReplaceAll(block, "{{TOGGLE_KEY}}", toggleKey)
}

func normalizeToggleKey(input string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(input))
	if key == "" {
		return defaultToggleKey, nil
	}
	if len(key) > 32 {
		return "", fmt.Errorf("按键名称过长")
	}
	switch key {
	case "SPACE":
		return "", fmt.Errorf("不能使用 SPACE，空格需要保留为跳跃键")
	case "INS":
		return "", fmt.Errorf("不能使用 INS，INS 已保留为 wait 检测键")
	case "SHIFT":
		return "", fmt.Errorf("不能使用 SHIFT，SHIFT 需要保留为 +speed")
	}
	for _, r := range key {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return "", fmt.Errorf("只支持字母、数字和下划线，例如 KP_5")
	}
	return key, nil
}

func cleanCFGAsync(cfgDir string) {
	if !isCfgDir(cfgDir) {
		postStatus("路径无效：请选择 Left 4 Dead 2\\left4dead2\\cfg 目录。")
		return
	}
	autoexec := filepath.Join(cfgDir, "autoexec.cfg")
	original, err := os.ReadFile(autoexec)
	if err != nil {
		postStatus("读取失败：" + err.Error())
		return
	}
	next := removeBlock(string(original))
	if next == string(original) {
		postStatus("未发现本工具写入的 CFG 标记块，无需清理。")
		return
	}
	backup := filepath.Join(cfgDir, fmt.Sprintf("autoexec.cfg.cleanbak.%s", time.Now().Format("20060102-150405")))
	if err := os.WriteFile(backup, original, 0644); err != nil {
		postStatus("清理前备份失败：" + err.Error())
		return
	}
	if err := os.WriteFile(autoexec, []byte(strings.TrimSpace(next)+"\r\n"), 0644); err != nil {
		postStatus("清理失败：" + err.Error())
		return
	}
	postStatus("清理完成：已删除本工具写入的 CFG 块，并保留清理前备份。请手动从 Steam 启动项移除：+exec autoexec.cfg")
}

func openCfgDirAsync(cfgDir string) {
	if !isCfgDir(cfgDir) {
		postStatus("路径无效，无法打开。")
		return
	}
	procShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16Ptr("open"))), uintptr(unsafe.Pointer(utf16Ptr(cfgDir))), 0, 0, SW_SHOW)
	postStatus("已请求打开 cfg 目录。")
}

func removeBlock(s string) string {
	start := strings.Index(s, blockBegin)
	end := strings.Index(s, blockEnd)
	if start < 0 || end < 0 || end < start {
		return s
	}
	end += len(blockEnd)
	for end < len(s) && (s[end] == '\r' || s[end] == '\n') {
		end++
	}
	return strings.TrimRight(s[:start], "\r\n") + "\r\n" + s[end:]
}

func firstDetectedCfgDir() string {
	for _, dir := range candidateCfgDirs() {
		if isCfgDir(dir) {
			return dir
		}
	}
	return ""
}

func candidateCfgDirs() []string {
	var out []string
	for _, base := range []string{
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("ProgramFiles"),
	} {
		if base != "" {
			out = append(out, filepath.Join(base, "Steam", "steamapps", "common", "Left 4 Dead 2", "left4dead2", "cfg"))
		}
	}
	for _, drive := range logicalDrives() {
		out = append(out,
			filepath.Join(drive, "SteamLibrary", "steamapps", "common", "Left 4 Dead 2", "left4dead2", "cfg"),
			filepath.Join(drive, "Steam", "steamapps", "common", "Left 4 Dead 2", "left4dead2", "cfg"),
		)
	}
	return dedupe(out)
}

func logicalDrives() []string {
	mask, _, _ := procGetLogicalDrives.Call()
	var drives []string
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) != 0 {
			drives = append(drives, fmt.Sprintf("%c:\\", 'A'+i))
		}
	}
	return drives
}

func isCfgDir(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return false
	}
	normalized := strings.ToLower(filepath.Clean(path))
	return strings.HasSuffix(normalized, filepath.Join("left4dead2", "cfg"))
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range in {
		if v == "" {
			continue
		}
		key := strings.ToLower(filepath.Clean(v))
		if !seen[key] {
			seen[key] = true
			out = append(out, v)
		}
	}
	return out
}

func setStatus(text string) {
	setWindowText(controls[idStatus], text)
}

func copyToClipboard(text string) bool {
	if ok, _, _ := procOpenClipboard.Call(mainHwnd); ok == 0 {
		return false
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()

	data := syscall.StringToUTF16(text)
	size := uintptr(len(data) * 2)
	mem, _, _ := procGlobalAlloc.Call(GMEM_MOVEABLE, size)
	if mem == 0 {
		return false
	}
	ptr, _, _ := procGlobalLock.Call(mem)
	if ptr == 0 {
		procGlobalFree.Call(mem)
		return false
	}
	procLstrcpyW.Call(ptr, uintptr(unsafe.Pointer(&data[0])))
	procGlobalUnlock.Call(mem)

	ok, _, _ := procSetClipboardData.Call(CF_UNICODETEXT, mem)
	if ok == 0 {
		procGlobalFree.Call(mem)
	}
	return ok != 0
}

func postStatus(text string) {
	uiMu.Lock()
	pendingText = text
	uiMu.Unlock()
	if mainHwnd != 0 {
		procPostMessageW.Call(mainHwnd, wmAppStatus, 0, 0)
	}
}

func postPath(path, text string) {
	uiMu.Lock()
	pendingPath = path
	pendingText = text
	uiMu.Unlock()
	if mainHwnd != 0 {
		procPostMessageW.Call(mainHwnd, wmAppPath, 0, 0)
	}
}

func takePendingStatus() string {
	uiMu.Lock()
	defer uiMu.Unlock()
	text := pendingText
	pendingText = ""
	return text
}

func takePendingPath() (string, string) {
	uiMu.Lock()
	defer uiMu.Unlock()
	path := pendingPath
	text := pendingText
	pendingPath = ""
	pendingText = ""
	return path, text
}

func setWindowText(hwnd uintptr, text string) {
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(utf16Ptr(text))))
}

func getControlText(hwnd uintptr) string {
	length := send(hwnd, WM_GETTEXTLENGTH, 0, 0)
	buf := make([]uint16, length+1)
	send(hwnd, WM_GETTEXT, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf)
}

func send(hwnd uintptr, message uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := procSendMessageW.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}

func utf16Ptr(s string) *uint16 {
	p, _ := syscall.UTF16PtrFromString(s)
	return p
}

func copyUTF16(dst []uint16, s string) {
	src := syscall.StringToUTF16(s)
	if len(src) > len(dst) {
		src = src[:len(dst)]
		src[len(src)-1] = 0
	}
	copy(dst, src)
}

func neg(v uintptr) uintptr {
	return ^(v - 1)
}
