package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/audio"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

func main() {
	nsgo.Run(Root, nsgo.WindowSize(1010, 740), nsgo.WindowConstraints(core.WindowConstraints{MinWidth: 480, MinHeight: 360}))
}

type label func(zh, en string) string

func Root(cx *core.Scope) core.Node {
	theme := core.DefaultTheme()
	theme.Light.Background = core.RGB(0xf7f5ef)
	theme.Light.Surface = core.RGB(0xfffefa)
	theme.Light.Text = core.RGB(0x302e28)
	theme.Light.TextMuted = core.RGB(0x68665d)
	theme.Light.Accent = core.RGB(0x763b32)
	theme.Light.AccentHover = core.RGB(0x69332c)
	theme.Light.AccentPressed = core.RGB(0x572a24)
	theme.Light.Focus = core.RGBA(0x763b32, 0.65)
	theme.Light.Selection = core.RGBA(0x763b32, 0.2)
	theme.Light.Border = core.RGB(0xdedbd1)
	theme.Dark.Background = core.RGB(0x22211f)
	theme.Dark.Surface = core.RGB(0x2d2b28)
	theme.Dark.Text = core.RGB(0xf5f0e7)
	theme.Dark.TextMuted = core.RGB(0xc4bdb0)
	theme.Dark.Border = core.RGB(0x504b44)
	theme.Dark.Accent = core.RGB(0xc98673)
	theme.Dark.AccentHover = core.RGB(0xd59987)
	theme.Dark.AccentPressed = core.RGB(0xb57667)
	theme.Dark.AccentText = core.RGB(0x231b17)
	cx.SetTheme(theme)
	prefs := readingPreferences(cx)
	desktop.SetTitle(cx, "Owl Dictionary")
	l := label(func(zh, en string) string {
		if strings.HasPrefix(strings.ToLower(cx.Env().Locale()), "zh") {
			return zh
		}
		return en
	})
	mode := core.NewPersisted(cx, "source-mode", "local", core.PersistOptions{})
	path := core.NewPersisted(cx, "mdx-file", "", core.PersistOptions{})
	server := core.NewPersisted(cx, "server-url", "", core.PersistOptions{})
	desktop.OnOpenFiles(cx, func(paths []string) {
		for _, p := range paths {
			if strings.EqualFold(filepath.Ext(p), ".mdx") {
				_ = path.Set(p)
				_ = mode.Set("local")
				break
			}
		}
	})
	return core.View{StyleFunc: func() core.Style {
		gutter := core.Points(24)
		if cx.Env().Window().Width < 650 {
			gutter = 14
		}
		return core.Style{Width: core.Pct(100), MaxWidth: core.Pt(1280), AlignSelf: core.AlignCenter, Height: core.Pct(100), Padding: core.All(gutter), Gap: 14, Background: cx.Palette().Background, Color: cx.Palette().Text}
	}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 10}, Children: []core.Node{
			core.View{Style: rowStyle.Merge(core.Style{FlexGrow: 1}), Children: []core.Node{
				core.Image{Source: core.ImageSource{Data: owlIcon}, Style: core.Style{Width: core.Pt(38), Height: core.Pt(38)}},
				title("Owl", 26),
			}},
			ui.ToggleGroup{Label: l("词典来源", "Dictionary source"), Children: []core.Node{
				ui.ToggleButton{Icon: lucide.BookOpen, Text: l("本地词典", "Local dictionary"), TestID: "local", On: func() bool { return mode.Get() != "server" }, OnChange: func(bool) { _ = mode.Set("local") }},
				ui.ToggleButton{Icon: lucide.Cloud, Text: l("Owl 服务端", "Owl server"), TestID: "server", On: func() bool { return mode.Get() == "server" }, OnChange: func(bool) { _ = mode.Set("server") }},
			}},
			preferenceMenu(cx, l, prefs),
		}},
		rule(cx),
		core.Show{When: func() bool { return mode.Get() == "server" }, Then: func(cx *core.Scope) core.Node { return serverPane(cx, l, prefs, server) }, Else: func(cx *core.Scope) core.Node { return localPane(cx, l, prefs, path) }},
	}}
}

var paneStyle = core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 12}
var rowStyle = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}
var inputStyle = core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(36)}

func localPane(cx *core.Scope, l label, prefs *readingPrefs, path *core.Persisted[string]) core.Node {
	note := core.NewSignal(cx, "")
	book := core.NewResource(cx, func() (string, bool) { p := path.Get(); return p, p != "" }, func(ctx context.Context, p string, _ core.ResourceInfo) (*localBook, error) { return openBook(ctx, p) })
	pick := func() {
		cx.PickFiles(core.PickFilesOptions{Title: l("打开 MDX 词典", "Open an MDX dictionary"), Types: []string{".mdx"}, TypesName: "MDX"}, func(files []core.File, err error) {
			if err != nil {
				note.Set(err.Error())
				return
			}
			if len(files) == 0 {
				return
			}
			note.Set("")
			desktop.AddRecentFile(cx, files[0].Path)
			if path.Get() == files[0].Path {
				book.Refetch()
			} else if err := path.Set(files[0].Path); err != nil {
				note.Set(err.Error())
			}
		})
	}
	cx.OnKey("mod+o", pick)
	openButton := ui.Button{Text: l("打开词典…", "Open dictionary…"), Icon: lucide.FolderOpen, TestID: "open-mdx", OnPress: pick, Size: ui.SizeLarge, Style: core.Style{AlignSelf: core.AlignStart}}
	return core.View{Style: paneStyle, Children: []core.Node{
		core.Show{When: func() bool { return note.Get() != "" }, Then: func(*core.Scope) core.Node { return core.Text{ContentFunc: note.Get} }},
		core.Dynamic(func(cx *core.Scope) core.Node {
			if book.Loading() {
				return setupPage(cx, lucide.BookOpen, l("正在打开词典", "Opening dictionary"), filepath.Base(path.Get()), ui.Spinner{})
			}
			if err := book.Err(); err != nil {
				return setupPage(cx, lucide.BookOpen, l("无法打开词典", "Could not open dictionary"), err.Error(), openButton)
			}
			if b, ok := book.Get(); ok {
				change := openButton
				change.Variant = ui.ButtonGhost
				change.Size = ui.SizeDefault
				return reader(cx, l, prefs, b, readerSource{title: cmp.Or(b.dict.Title(), b.dict.Name()), subtitle: fmt.Sprintf(l("本地词典 · %d 个词头", "Local dictionary · %d headwords"), len(b.words)), key: "local:" + path.Get(), action: change})
			}
			return setupPage(cx, lucide.BookOpenText, l("打开本地词典", "Open a local dictionary"),
				l("选择 MDX 文件即可离线查词。把配套的 MDD 文件放在同一目录，即可查看图片和播放发音。", "Choose an MDX file to read offline. Keep its MDD volumes in the same folder for illustrations and pronunciation."), openButton)
		}),
	}}
}

type loginRequest struct{ address, user, password string }

func serverPane(cx *core.Scope, l label, prefs *readingPrefs, saved *core.Persisted[string]) core.Node {
	address := core.NewSignal(cx, cmp.Or(saved.Get(), "http://localhost:8080"))
	account := core.NewSignal(cx, false)
	user := core.NewSignal(cx, "")
	password := core.NewSignal(cx, "")
	request := core.NewSignal[*loginRequest](cx, nil)
	server := core.NewResource(cx, func() (*loginRequest, bool) { r := request.Get(); return r, r != nil }, func(ctx context.Context, r *loginRequest, _ core.ResourceInfo) (*remoteServer, error) {
		return connect(ctx, r.address, r.user, r.password)
	})
	submit := func() {
		r := &loginRequest{address: strings.TrimSpace(address.Get())}
		if r.address == "" || server.Loading() {
			return
		}
		if account.Get() {
			r.user = strings.TrimSpace(user.Get())
			r.password = password.Get()
			if r.user == "" || r.password == "" {
				return
			}
		}
		request.Set(r)
	}
	return core.Dynamic(func(cx *core.Scope) core.Node {
		if request.Get() != nil && server.State() == core.ResourceReady {
			s, _ := server.Get()
			_ = saved.Set(s.base.String())
			password.Set("")
			name := l("游客", "Guest")
			if s.token != "" {
				name = user.GetUntracked()
			}
			return reader(cx, l, prefs, s, readerSource{title: s.base.Host, subtitle: l("服务器 · ", "Server · ") + name, key: "server:" + s.base.String() + "/" + request.Get().user,
				action: ui.Button{Text: l("断开连接", "Disconnect"), Icon: lucide.Unplug, Variant: ui.ButtonGhost, TestID: "disconnect", OnPress: func() { request.Set(nil) }}})
		}
		return setupPage(cx, lucide.Cloud, l("连接 Owl 服务端", "Connect to Owl"),
			l("以游客身份查阅公开词典，或使用账号登录，查询自己的私有词库。", "Read public dictionaries as a guest, or sign in to include your private library."),
			core.Text{Content: l("服务端地址", "Server address"), Style: ui.TypeBody},
			ui.Input{Value: address.Get, OnChange: address.Set, Label: l("服务端地址", "Server address"), NoAutoCorrect: true, AutoFocus: true, TestID: "address", OnSubmit: func(string) { submit() }},
			core.View{Style: core.Style{AlignSelf: core.AlignStart}, Children: []core.Node{
				ui.ToggleButton{Text: l("使用账号登录", "Sign in with an account"), On: account.Get, OnChange: account.Set, TestID: "use-account"},
			}},
			core.Show{When: account.Get, Then: func(*core.Scope) core.Node {
				return core.View{Style: core.Style{Gap: 8}, Children: []core.Node{
					ui.Input{Label: l("用户名", "Username"), Placeholder: l("用户名", "Username"), Value: user.Get, OnChange: user.Set, NoAutoCorrect: true, TestID: "username"},
					ui.Input{Label: l("密码", "Password"), Placeholder: l("密码", "Password"), Value: password.Get, OnChange: password.Set, Masked: true, TestID: "password", OnSubmit: func(string) { submit() }},
				}}
			}},
			ui.Button{Text: l("连接", "Connect"), TestID: "connect", Busy: server.Loading, Style: core.Style{AlignSelf: core.AlignStart}, Disabled: func() bool {
				return strings.TrimSpace(address.Get()) == "" || account.Get() && (strings.TrimSpace(user.Get()) == "" || password.Get() == "")
			}, OnPress: submit},
			core.Text{ContentFunc: func() string {
				if err := server.Err(); request.Get() != nil && err != nil {
					var network net.Error
					switch {
					case errors.Is(err, errServerAddress):
						return l("地址须以 http:// 或 https:// 开头，例如 https://owl.example.org。", "Enter a full address, such as https://owl.example.org.")
					case errors.Is(err, errUnauthorized):
						return l("登录失败。请检查用户名和密码后重试。", "Sign-in failed. Check your username and password and try again.")
					case errors.As(err, &network):
						return l("无法连接服务端。请检查地址和网络后重试。", "Could not reach the server. Check the address and your connection.")
					default:
						return err.Error()
					}
				}
				return ""
			}, StyleFunc: func() core.Style { return core.Style{Color: cx.Palette().Accent} }, Role: core.RoleStatus, TestID: "connection-status"},
		)
	})
}

func attachmentView(cx *core.Scope, l label, s source, item entry, a attachment) core.Node {
	asked := core.NewSignal(cx, a.kind == "image")
	data := core.NewResource(cx, func() (string, bool) { return a.ref, asked.Get() }, func(ctx context.Context, ref string, _ core.ResourceInfo) ([]byte, error) {
		return s.asset(ctx, item, ref)
	})
	return core.Dynamic(func(cx *core.Scope) core.Node {
		if err := data.Err(); err != nil && !errors.Is(err, context.Canceled) {
			return core.Text{Content: fmt.Sprintf("%s · %s", l("媒体加载失败", "Media unavailable"), err)}
		}
		if bytes, ok := data.Get(); ok {
			if a.kind == "image" {
				size := core.NewSignal(cx, core.Size{Width: 160, Height: 100})
				return core.Image{
					Source: core.ImageSource{Data: bytes}, Label: cmp.Or(a.label, item.Word),
					ResizeMode: core.ScaleDown, TestID: "entry-image",
					OnLoad: func(pixels core.Size) {
						if pixels.Width > 0 && pixels.Height > 0 {
							size.Set(pixels)
						}
					},
					StyleFunc: func() core.Style {
						pixels := size.Get()
						width := pixels.Width * min(1, 320/pixels.Height)
						return core.Style{Width: core.Pt(width), MaxWidth: core.Pct(100), AspectRatio: float32(pixels.Width / pixels.Height), AlignSelf: core.AlignStart}
					},
				}
			}
			status := core.NewSignal(cx, "")
			player := audio.NewPlayer(cx, audio.Source{Data: bytes}, audio.PlayerOptions{OnError: func(err error) { status.Set(err.Error()) }})
			play := func() {
				player.Play(func(err error) {
					if err != nil {
						status.Set(err.Error())
					}
				})
			}
			play()
			return core.View{Style: rowStyle, Children: []core.Node{ui.Button{Icon: lucide.Volume2, Text: l("播放发音", "Play audio"), Variant: ui.ButtonSecondary, OnPress: play}, core.Text{ContentFunc: status.Get}}}
		}
		if a.kind == "audio" {
			return ui.Button{Icon: lucide.Volume2, Text: l("播放发音", "Play audio"), Variant: ui.ButtonSecondary, Style: core.Style{AlignSelf: core.AlignStart}, Busy: data.Loading, OnPress: func() { asked.Set(true) }}
		}
		return core.Text{Content: cmp.Or(a.label, l("正在读取图片…", "Loading image…")), Style: ui.TypeCaption}
	})
}
