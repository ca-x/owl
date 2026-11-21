package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/nsgo-dev/nsgo"
	"github.com/nsgo-dev/nsgo/audio"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/desktop"
	"github.com/nsgo-dev/nsgo/ui"
)

func main() {
	nsgo.Run(Root, nsgo.WindowSize(780, 620), nsgo.WindowConstraints(core.WindowConstraints{MinWidth: 460, MinHeight: 360}))
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
	cx.SetTheme(theme)
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
		return core.Style{Width: core.Pct(100), Height: core.Pct(100), Padding: core.All(24), Gap: 16, Background: cx.Palette().Background, Color: cx.Palette().Text}
	}, Children: []core.Node{
		core.View{Style: core.Style{Direction: core.Row, AlignItems: core.AlignCenter, JustifyContent: core.JustifySpaceBetween}, Children: []core.Node{
			core.Text{Content: "Owl", Style: ui.TypeHeading.Merge(core.Style{FontFamily: "Georgia, serif", FontSize: 28, LineHeight: 34})},
			ui.ToggleGroup{Label: l("词典来源", "Dictionary source"), Children: []core.Node{
				ui.ToggleButton{Text: l("本地词典", "Local dictionary"), TestID: "local", On: func() bool { return mode.Get() != "server" }, OnChange: func(bool) { _ = mode.Set("local") }},
				ui.ToggleButton{Text: l("Owl 服务端", "Owl server"), TestID: "server", On: func() bool { return mode.Get() == "server" }, OnChange: func(bool) { _ = mode.Set("server") }},
			}},
		}},
		core.Show{When: func() bool { return mode.Get() == "server" }, Then: func(cx *core.Scope) core.Node { return serverPane(cx, l, server) }, Else: func(cx *core.Scope) core.Node { return localPane(cx, l, path) }},
	}}
}

var paneStyle = core.Style{FlexGrow: 1, FlexShrink: 1, Gap: 12}
var rowStyle = core.Style{Direction: core.Row, AlignItems: core.AlignCenter, Gap: 8}
var inputStyle = core.Style{FlexGrow: 1, FlexShrink: 1, MinWidth: core.Pt(0), Height: core.Pt(36)}

func localPane(cx *core.Scope, l label, path *core.Persisted[string]) core.Node {
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
	openButton := ui.Button{Text: l("打开词典…", "Open dictionary…"), TestID: "open-mdx", OnPress: pick, Style: core.Style{AlignSelf: core.AlignStart}}
	return core.View{Style: paneStyle, Children: []core.Node{
		core.Show{When: func() bool { return path.Get() != "" }, Then: func(*core.Scope) core.Node {
			changeBook := openButton
			changeBook.Variant = ui.ButtonSecondary
			return core.View{Style: rowStyle, Children: []core.Node{
				changeBook,
				core.Text{ContentFunc: func() string { return filepath.Base(path.Get()) }, Style: ui.TypeBody.Merge(core.Style{FlexShrink: 1}), MaxLines: 1},
			}}
		}},
		core.Show{When: func() bool { return note.Get() != "" }, Then: func(*core.Scope) core.Node {
			return core.Text{ContentFunc: note.Get}
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			if book.Loading() {
				return core.Text{Content: l("正在读取词典索引…", "Reading the dictionary index…")}
			}
			if err := book.Err(); err != nil {
				return core.Text{Content: err.Error()}
			}
			if b, ok := book.Get(); ok {
				return reader(cx, l, b)
			}
			return core.View{Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter, Gap: 14, MaxWidth: core.Pt(450)}, Children: []core.Node{
				core.Text{Content: l("打开本地词典", "Open a local dictionary"), Style: ui.TypeHeading.Merge(core.Style{FontFamily: "Georgia, serif", FontSize: 27, LineHeight: 34})},
				core.Text{Content: l("选择 MDX 文件即可离线查词。把配套的 MDD 文件放在同一目录，即可查看图片和播放发音。", "Choose an MDX file to read offline. Keep its MDD files in the same folder for illustrations and audio."), Style: ui.TypeBody.Merge(core.Style{FontSize: 15, LineHeight: 23})},
				openButton,
			}}
		}),
	}}
}

type loginRequest struct{ address, user, password string }

func serverPane(cx *core.Scope, l label, saved *core.Persisted[string]) core.Node {
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
			return core.View{Style: paneStyle, Children: []core.Node{
				core.View{Style: rowStyle, Children: []core.Node{
					core.Text{Content: s.base.Host + " · " + name, Style: ui.TypeBody.Merge(core.Style{FlexGrow: 1}), MaxLines: 1},
					ui.Button{Text: l("断开连接", "Disconnect"), Variant: ui.ButtonSecondary, TestID: "disconnect", OnPress: func() { request.Set(nil) }},
				}},
				reader(cx, l, s),
			}}
		}
		return core.View{Style: paneStyle, Children: []core.Node{
			core.Text{Content: l("服务端地址", "Server address"), Style: ui.TypeBody},
			ui.Input{Value: address.Get, OnChange: address.Set, Label: l("服务端地址", "Server address"), NoAutoCorrect: true, AutoFocus: true, TestID: "address", OnSubmit: func(string) { submit() }},
			ui.ToggleButton{Text: l("使用账号登录", "Sign in with an account"), On: account.Get, OnChange: account.Set, TestID: "use-account"},
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
					return err.Error()
				}
				return ""
			}, TestID: "connection-status"},
		}}
	})
}

type lookupRequest struct{ word string }

func reader(cx *core.Scope, l label, s source) core.Node {
	word := core.NewSignal(cx, "")
	request := core.NewSignal[*lookupRequest](cx, nil)
	result := core.NewResource(cx, func() (*lookupRequest, bool) { r := request.Get(); return r, r != nil }, func(ctx context.Context, r *lookupRequest, _ core.ResourceInfo) ([]entry, error) {
		return s.lookup(ctx, r.word)
	})
	run := func(q string) {
		if q = strings.TrimSpace(q); q != "" {
			word.Set(q)
			request.Set(&lookupRequest{q})
		}
	}
	input := new(core.Ref)
	cx.OnKey("mod+f", input.Focus)
	return core.View{Style: paneStyle, Children: []core.Node{
		core.View{Style: rowStyle, Children: []core.Node{
			ui.Input{Ref: input, Style: inputStyle, Value: word.Get, OnChange: word.Set, OnSubmit: run, AutoFocus: true, NoAutoCorrect: true, Role: core.RoleSearchField, Label: l("查询词头", "Headword"), Placeholder: l("输入单词、词组或汉字", "Enter a word, phrase, or character"), TestID: "query"},
			ui.Button{Text: l("查词", "Look up"), TestID: "lookup", Style: core.Style{Height: core.Pt(36)}, Disabled: func() bool { return strings.TrimSpace(word.Get()) == "" }, OnPress: func() { run(word.Get()) }},
		}},
		core.Dynamic(func(cx *core.Scope) core.Node {
			if result.Loading() {
				return core.Text{Content: l("正在查词…", "Looking up…")}
			}
			if err := result.Err(); err != nil {
				return core.Text{Content: err.Error(), TestID: "lookup-error"}
			}
			items, ok := result.Get()
			if !ok {
				return core.View{}
			}
			if len(items) == 0 {
				return core.Text{Content: l("没有找到匹配结果", "No matching entries") + " · " + request.Get().word, TestID: "no-match"}
			}
			selected := core.NewSignal(cx, 0)
			options := make([]ui.Option[int], len(items))
			for i, item := range items {
				options[i] = ui.Option[int]{Value: i, Label: item.Word + " · " + item.Dictionary}
			}
			var controls []core.Node
			if len(items) > 1 {
				controls = append(controls, ui.Select[int]{Label: l("匹配词条", "Matching entries"), Options: options, Value: selected.Get, OnChange: selected.Set, TestID: "matches", Style: core.Style{Width: core.Pct(100)}})
			}
			controls = append(controls, core.Dynamic(func(cx *core.Scope) core.Node { return entryView(cx, l, s, items[selected.Get()], run) }))
			return core.View{Style: paneStyle, Children: controls}
		}),
	}}
}

func entryView(cx *core.Scope, l label, s source, item entry, lookup func(string)) core.Node {
	def := readDefinition(item.HTML)
	copyLabel := core.NewSignal(cx, l("复制释义", "Copy definition"))
	kids := []core.Node{
		core.Text{Content: item.Dictionary, Style: ui.TypeCaption},
		core.Text{Content: item.Word, Style: ui.TypeHeading.Merge(core.Style{FontFamily: "Georgia, serif", FontSize: 30, LineHeight: 38}), Selectable: core.SelectableOn, TestID: "headword"},
		core.Text{Content: def.text, Style: ui.TypeBody.Merge(core.Style{FontSize: 16, LineHeight: 25}), Selectable: core.SelectableOn, TestID: "definition"},
	}
	for _, a := range def.attachments {
		kids = append(kids, attachmentView(cx, l, s, item, a))
	}
	for _, word := range def.related {
		kids = append(kids, ui.Button{Text: "→ " + word, Variant: ui.ButtonGhost, Style: core.Style{AlignSelf: core.AlignStart}, OnPress: func() { lookup(word) }})
	}
	kids = append(kids, core.Dynamic(func(*core.Scope) core.Node {
		return ui.Button{Text: copyLabel.Get(), TestID: "copy", Variant: ui.ButtonSecondary, Style: core.Style{AlignSelf: core.AlignStart}, OnPress: func() {
			cx.SetClipboard(item.Word + " · " + item.Dictionary + "\n\n" + def.text)
			copyLabel.Set(l("已复制", "Copied"))
		}}
	}))
	return core.Scroll{Style: core.Style{FlexGrow: 1, FlexShrink: 1}, Children: []core.Node{core.View{Style: core.Style{Gap: 12, Padding: core.All(8)}, Children: kids}}}
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
			return core.View{Style: rowStyle, Children: []core.Node{ui.Button{Text: l("播放发音", "Play audio"), Variant: ui.ButtonSecondary, OnPress: play}, core.Text{ContentFunc: status.Get}}}
		}
		if a.kind == "audio" {
			return ui.Button{Text: l("播放发音", "Play audio"), Variant: ui.ButtonSecondary, Style: core.Style{AlignSelf: core.AlignStart}, Busy: data.Loading, OnPress: func() { asked.Set(true) }}
		}
		return core.Text{Content: cmp.Or(a.label, l("正在读取图片…", "Loading image…")), Style: ui.TypeCaption}
	})
}
