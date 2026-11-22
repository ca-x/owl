package main

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nsgo-dev/nsgo/base"
	"github.com/nsgo-dev/nsgo/base/textview"
	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

type readerSource struct {
	title, subtitle, key string
	action               ui.Button
}

type lookupKey struct {
	word  string
	scope int
}

type readingDesk struct {
	cx                      *core.Scope
	tr                      label
	prefs                   *readingPrefs
	source                  source
	info                    readerSource
	input                   core.Ref
	word                    *core.Signal[string]
	scope, selected, active *core.Signal[int]
	activeSource            *core.Signal[int]
	editing, historyOpen    *core.Signal[bool]
	original, workspaceOpen *core.Signal[bool]
	current                 *core.Signal[*lookupKey]
	results                 *core.Resource[[]entry]
	suggestions             *core.Resource[[]suggestion]
	recent                  *core.Persisted[[]string]
	trail                   []lookupKey
	position                *core.Signal[int]
}

func reader(cx *core.Scope, tr label, prefs *readingPrefs, s source, info readerSource) core.Node {
	d := &readingDesk{cx: cx, tr: tr, prefs: prefs, source: s, info: info,
		word: core.NewSignal(cx, ""), scope: core.NewSignal(cx, 0), selected: core.NewSignal(cx, 0), active: core.NewSignal(cx, -1),
		editing: core.NewSignal(cx, false), historyOpen: core.NewSignal(cx, false), current: core.NewSignal[*lookupKey](cx, nil), position: core.NewSignal(cx, -1),
		original: core.NewSignal(cx, false), workspaceOpen: core.NewSignal(cx, false),
		activeSource: core.NewSignal(cx, -1),
		recent:       core.NewPersisted(cx, "recent:"+info.key, []string{}, core.PersistOptions{}),
	}
	d.results = core.NewResource(cx, func() (*lookupKey, bool) { q := d.current.Get(); return q, q != nil }, func(ctx context.Context, q *lookupKey, _ core.ResourceInfo) ([]entry, error) {
		return s.lookup(ctx, q.word, q.scope)
	})
	d.suggestions = core.NewResource(cx, func() (lookupKey, bool) {
		q := lookupKey{strings.TrimSpace(d.word.Get()), d.scope.Get()}
		return q, d.editing.Get() && q.word != ""
	}, func(ctx context.Context, q lookupKey, _ core.ResourceInfo) ([]suggestion, error) {
		if _, remote := s.(*remoteServer); remote {
			timer := time.NewTimer(150 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		return s.suggest(ctx, q.word, q.scope)
	})
	cx.Effect(func() {
		q := d.current.Get()
		if q == nil || d.results.State() != core.ResourceReady {
			return
		}
		if items, ok := d.results.Get(); ok && len(items) > 0 {
			_ = d.recent.Update(func(old []string) []string {
				next := []string{q.word}
				for _, w := range old {
					if !strings.EqualFold(w, q.word) && len(next) < 8 {
						next = append(next, w)
					}
				}
				return next
			})
		}
	})
	cx.OnKey("mod+f", d.input.Focus)
	cx.OnKey("alt+left", func() { d.move(-1) })
	cx.OnKey("alt+right", func() { d.move(1) })
	paper := func(cx *core.Scope) core.Node {
		return core.View{Style: paneStyle, Children: []core.Node{d.searchBar(), d.workspace(cx)}}
	}
	if server, ok := s.(*remoteServer); ok {
		return core.Show{When: d.workspaceOpen.Get, Then: func(cx *core.Scope) core.Node { return d.serverWorkspace(cx, server) }, Else: paper}
	}
	return paper(cx)
}

func (d *readingDesk) show(q lookupKey) {
	d.editing.Set(false)
	d.historyOpen.Set(false)
	d.active.Set(-1)
	d.activeSource.Set(-1)
	d.selected.Set(0)
	d.word.Set(q.word)
	d.scope.Set(q.scope)
	d.current.Set(&q)
}

func (d *readingDesk) search(word string) {
	d.searchIn(word, d.scope.Get())
}

func (d *readingDesk) searchIn(word string, scope int) {
	q := lookupKey{strings.TrimSpace(word), scope}
	if q.word == "" {
		return
	}
	idx := d.position.Get()
	if idx < 0 || d.trail[idx] != q {
		d.trail = append(d.trail[:idx+1], q)
		d.position.Set(len(d.trail) - 1)
	}
	d.show(q)
}

func (d *readingDesk) move(by int) {
	idx := d.position.Get() + by
	if idx < 0 || idx >= len(d.trail) {
		return
	}
	d.position.Set(idx)
	d.show(d.trail[idx])
}

func (d *readingDesk) offered() []suggestion {
	if !d.editing.Get() || strings.TrimSpace(d.word.Get()) == "" || d.suggestions.State() != core.ResourceReady {
		return nil
	}
	words, _ := d.suggestions.Get()
	return words
}

func (d *readingDesk) searchBar() core.Node {
	tr, cx := d.tr, d.cx
	left := core.View{Style: rowStyle.Merge(core.Style{FlexGrow: 1, FlexShrink: 1}), Children: []core.Node{
		core.Icon{SVG: lucide.Library(), StyleFunc: func() core.Style { return core.Style{FontSize: 20, Color: cx.Palette().Accent} }},
		core.View{Style: core.Style{FlexShrink: 1, Gap: 2}, Children: []core.Node{
			core.Text{Content: d.info.title, Style: ui.TypeTitle, MaxLines: 1}, mutedText(cx, d.info.subtitle),
		}},
	}}
	query := ui.Input{Ref: &d.input, Style: inputStyle.Merge(core.Style{Height: core.Pt(42), FontSize: 16}), Value: d.word.Get,
		OnChange: func(q string) { d.word.Set(q); d.editing.Set(true); d.active.Set(-1); d.activeSource.Set(-1) },
		OnKeyPress: func(key string) {
			words := d.offered()
			switch strings.ToLower(key) {
			case "arrowdown", "down":
				if len(words) > 0 {
					d.active.Set((d.active.Get() + 1) % len(words))
					d.activeSource.Set(-1)
				}
			case "arrowup", "up":
				if len(words) > 0 {
					i := d.active.Get() - 1
					if i < 0 {
						i = len(words) - 1
					}
					d.active.Set(i)
					d.activeSource.Set(-1)
				}
			case "arrowright", "right", "arrowleft", "left":
				if i := d.active.Get(); i >= 0 && i < len(words) {
					if strings.HasSuffix(strings.ToLower(key), "right") {
						d.activeSource.Set(min(d.activeSource.Get()+1, len(words[i].Sources)-1))
					} else {
						d.activeSource.Set(max(-1, d.activeSource.Get()-1))
					}
				}
			case "escape":
				d.editing.Set(false)
				d.active.Set(-1)
			}
		},
		OnSubmit: func(q string) {
			words := d.offered()
			if i := d.active.Get(); i >= 0 && i < len(words) {
				q = words[i].Word
				if src := d.activeSource.Get(); src >= 0 && src < len(words[i].Sources) {
					d.searchIn(q, words[i].Sources[src].ID)
					return
				}
			}
			d.search(q)
		},
		SubmitBehavior: core.SubmitBehaviorSubmit, AutoFocus: true, NoAutoCorrect: true, Role: core.RoleSearchField, Label: tr("查询词头", "Headword"), Placeholder: tr("输入单词、词组或汉字", "Enter a word, phrase, or character"), TestID: "query"}
	tools := []core.Node{left,
		ui.Button{Icon: lucide.ArrowLeft, Label: tr("上一次查询", "Previous search"), Variant: ui.ButtonGhost, TestID: "back", Disabled: func() bool { return d.position.Get() <= 0 }, OnPress: func() { d.move(-1) }},
		ui.Button{Icon: lucide.ArrowRight, Label: tr("下一次查询", "Next search"), Variant: ui.ButtonGhost, TestID: "forward", Disabled: func() bool { return d.position.Get() >= len(d.trail)-1 }, OnPress: func() { d.move(1) }},
		ui.Popover{Open: d.historyOpen.Get, OnOpenChange: func(open bool, _ base.Reason) { d.historyOpen.Set(open) }, Trigger: ui.Button{Icon: lucide.Clock3, Variant: ui.ButtonGhost}, Label: tr("最近查词", "Recent searches"), TestID: "history", Children: []core.Node{sectionTitle(cx, lucide.Clock3, tr("最近查词", "Recent searches")), core.Dynamic(func(*core.Scope) core.Node { return d.recentRows() })}},
	}
	if _, ok := d.source.(*remoteServer); ok {
		tools = append(tools, core.Dynamic(func(cx *core.Scope) core.Node {
			name := ""
			if cx.Env().Window().Width >= 740 {
				name = tr("工作区", "Workspace")
			}
			return ui.Button{Text: name, Icon: lucide.PanelsTopLeft, Label: tr("打开服务端工作区", "Open server workspace"), Variant: ui.ButtonGhost, TestID: "workspace", OnPress: func() { d.workspaceOpen.Set(true) }}
		}))
	}
	tools = append(tools, d.info.action)
	return core.View{StyleFunc: func() core.Style {
		s := pageStyle(cx, 16)
		s.Gap = 12
		return s
	}, Children: []core.Node{
		core.View{Style: rowStyle.Merge(core.Style{Gap: 4}), Children: tools},
		core.View{Style: rowStyle, Children: []core.Node{query, ui.Button{Text: tr("查词", "Look up"), Icon: lucide.Search, Size: ui.SizeLarge, Style: core.Style{Height: core.Pt(42)}, TestID: "lookup",
			Disabled: func() bool { return strings.TrimSpace(d.word.Get()) == "" }, OnPress: func() { d.search(d.word.Get()) }}}},
	}}
}

func (d *readingDesk) serverWorkspace(cx *core.Scope, s *remoteServer) core.Node {
	tr := d.tr
	address := s.base.String()
	uri := address
	var headers map[string]string
	if s.token != "" {
		uri = s.base.JoinPath("workspace").String()
		headers = map[string]string{"Authorization": "Bearer " + s.token}
	}
	message := core.NewSignal(cx, "")
	return core.View{Style: paneStyle, Children: []core.Node{
		core.View{Style: rowStyle, Children: []core.Node{
			ui.Button{Text: tr("返回查词", "Back to reader"), Icon: lucide.ArrowLeft, Variant: ui.ButtonSecondary, TestID: "close-workspace", OnPress: func() { d.workspaceOpen.Set(false) }},
			core.Text{Content: s.base.Host, MaxLines: 1, Style: ui.TypeLabel.Merge(core.Style{FlexGrow: 1, FlexShrink: 1})},
			ui.Button{Text: tr("在浏览器中打开", "Open in browser"), Icon: lucide.ExternalLink, Variant: ui.ButtonGhost, OnPress: func() { cx.OpenURL(address, nil) }},
		}},
		core.Show{When: func() bool { return message.Get() != "" }, Then: func(*core.Scope) core.Node {
			return core.Text{ContentFunc: message.Get, Role: core.RoleStatus}
		}},
		core.WebView{Source: core.WebSrc(uri), Headers: headers, Incognito: true, PinchZoom: true, Style: paneStyle, Label: tr("服务端工作区", "Server workspace"), TestID: "server-workspace", OnError: func(err error) { message.Set(err.Error()) }},
	}}
}

func (d *readingDesk) workspace(cx *core.Scope) core.Node {
	tr := d.tr
	var scopePicker core.Node

	if server, ok := d.source.(*remoteServer); ok {
		library := core.NewResource(cx, nil, func(ctx context.Context, _ int, _ core.ResourceInfo) ([]libraryBook, error) { return server.books(ctx) })
		scopePicker = core.Dynamic(func(*core.Scope) core.Node {
			books, ok := library.Get()
			if !ok || len(books) == 0 {
				return core.View{}
			}
			options := []ui.Option[int]{{Value: 0, Label: tr("全部可用词典", "All dictionaries")}}
			for _, b := range books {
				if b.Enabled {
					options = append(options, ui.Option[int]{Value: b.ID, Label: cmp.Or(b.Title, b.Name)})
				}
			}
			return ui.Select[int]{Value: d.scope.Get, Options: options, Label: tr("查询范围", "Dictionary scope"), TestID: "scope", Style: core.Style{MaxWidth: core.Pt(215), FlexShrink: 1},
				OnChange: func(i int) { d.scope.Set(i); d.search(d.word.Get()) }}
		})
	}
	index := func() core.Node {
		cols := []core.Node{}
		if scopePicker != nil {
			cols = append(cols, scopePicker)
		}
		cols = append(cols, sectionTitle(cx, lucide.ListFilter, tr("词目索引", "Word index")), rule(cx),
			core.Dynamic(func(*core.Scope) core.Node { return d.indexRows(false) }))
		return core.Scroll{Style: core.Style{Width: core.Pt(184), FlexShrink: 0}, Children: []core.Node{core.View{Style: core.Style{Gap: 10, Padding: core.Edges{Right: 10, Bottom: 14}}, Children: cols}}}
	}

	w := func() bool { return cx.Env().Window().Width >= 740 }
	return core.View{Style: paneStyle, Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			s := paneStyle
			if w() {
				s.Direction = core.Row
				s.Gap = 18
			}
			return s
		}, Children: []core.Node{
			core.Show{When: w, Then: func(*core.Scope) core.Node { return index() }, Else: func(*core.Scope) core.Node {
				n := core.Dynamic(func(*core.Scope) core.Node { return d.indexRows(true) })
				if scopePicker == nil {
					return n
				}
				return core.View{Style: core.Style{Gap: 5}, Children: []core.Node{scopePicker, n}}
			}},
			core.View{StyleFunc: func() core.Style {
				s := pageStyle(cx, 0)
				s.FlexGrow = 1
				s.FlexShrink = 1
				s.FlexBasis = core.Pt(0)
				s.MinHeight = core.Pt(0)
				s.MinWidth = core.Pt(0)
				s.Overflow = core.Hidden
				return s
			}, Children: []core.Node{core.Dynamic(func(cx *core.Scope) core.Node { return d.article(cx) })}},
		}},
	}}
}

func (d *readingDesk) indexRows(compact bool) core.Node {
	tr := d.tr
	var rows []core.Node
	if options := d.offered(); len(options) > 0 {
		rows = append(rows, mutedText(d.cx, tr("联想词", "Suggestions")))
		for i, word := range options {
			rows = append(rows, d.wordRow(word.Word, "", fmt.Sprintf("suggestion-%d", i), func() bool { return d.active.Get() == i && d.activeSource.Get() < 0 }, func() { d.search(word.Word) }))
			if len(word.Sources) > 0 {
				var sources []core.Node
				for j, src := range word.Sources {
					sources = append(sources, ui.ToggleButton{Text: src.Dictionary, Label: word.Word + " · " + src.Dictionary, TestID: fmt.Sprintf("suggestion-%d-source-%d", i, j),
						On: func() bool { return d.active.Get() == i && d.activeSource.Get() == j }, OnChange: func(bool) { d.searchIn(word.Word, src.ID) }})
				}
				rows = append(rows, core.View{Style: rowStyle.Merge(core.Style{Wrap: core.Wrap, Gap: 3}), Children: sources})
			}
		}
	} else if items, ok := d.results.Get(); d.current.Get() != nil && ok && len(items) > 0 {
		if compact {
			options := make([]ui.Option[int], len(items))
			for i, e := range items {
				options[i] = ui.Option[int]{Value: i, Label: e.Word + " · " + e.Dictionary}
			}
			return ui.Select[int]{Label: tr("匹配词条", "Matching entries"), Value: d.selected.Get, Options: options, OnChange: d.selected.Set, TestID: "matches"}
		}
		rows = append(rows, mutedText(d.cx, fmt.Sprintf(tr("%d 个匹配词条", "%d matching entries"), len(items))))
		for i, e := range items {
			detail := e.Dictionary
			if _, local := d.source.(*localBook); local {
				detail = ""
			}
			rows = append(rows, d.wordRow(e.Word, detail, fmt.Sprintf("match-%d", i), func() bool { return d.selected.Get() == i }, func() { d.selected.Set(i); d.editing.Set(false) }))
		}
	} else if local, ok := d.source.(*localBook); ok && !compact {
		rows = append(rows, mutedText(d.cx, tr("浏览词头", "Browse headwords")))
		for i, k := range local.words[:min(7, len(local.words))] {
			rows = append(rows, d.wordRow(k.word, "", fmt.Sprintf("browse-%d", i), nil, func() { d.search(k.word) }))
		}
	} else if !compact {
		return d.recentRows()
	}
	if len(rows) == 0 {
		return core.View{}
	}
	list := core.View{Style: core.Style{Gap: 3}, Children: rows}
	if compact {
		return core.Scroll{Style: core.Style{MaxHeight: core.Pt(140)}, Children: []core.Node{list}}
	}
	return list
}

func (d *readingDesk) wordRow(word, detail, id string, selected func() bool, press func()) core.Node {
	active := func() bool { return selected != nil && selected() }
	parts := []core.Node{core.Text{Content: word, Style: ui.TypeLabel, MaxLines: 1}}
	if detail != "" {
		t := mutedText(d.cx, detail)
		t.MaxLines = 1
		parts = append(parts, t)
	}
	return base.Button{Label: word, Selected: active, TestID: id, OnPress: func(core.PressEvent) { press() }, StyleFunc: func() core.Style {
		p := d.cx.Palette()
		s := core.Style{Gap: 3, Padding: core.Symmetric(8, 11), CornerRadius: 6, Borders: core.Edges{Left: 2}, BorderColor: core.Transparent, Hover: &core.Style{Background: p.SurfaceHover}, FocusVisible: &core.Style{OutlineWidth: 2, OutlineColor: p.Focus}}
		if active() {
			s.Background = ui.Mix(p.Background, p.Accent, .10)
			s.BorderColor = p.Accent
			s.Color = p.Accent
		}
		return s
	}, Children: parts}
}

func (d *readingDesk) recentRows() core.Node {
	var list []core.Node
	for i, word := range d.recent.Get() {
		list = append(list, d.wordRow(word, "", fmt.Sprintf("recent-%d", i), nil, func() { d.search(word) }))
	}
	if len(list) == 0 {
		return mutedText(d.cx, d.tr("还没有查词记录", "No recent searches"))
	}
	return core.View{Style: core.Style{Gap: 2}, Children: list}
}

func (d *readingDesk) article(cx *core.Scope) core.Node {
	tr := d.tr
	empty := func(icon func() *core.SVG, heading, detail string) core.Node {
		return core.View{Style: core.Style{FlexGrow: 1, JustifyContent: core.JustifyCenter, AlignItems: core.AlignCenter, Padding: core.All(22), Gap: 12}, Children: []core.Node{
			core.Icon{SVG: icon(), StyleFunc: func() core.Style { return core.Style{FontSize: 32, Color: cx.Palette().Accent} }},
			title(heading, 24), mutedText(cx, detail),
		}}
	}
	q := d.current.Get()
	if q == nil {
		return empty(lucide.BookOpenText, tr("从一个词开始", "Look up your first word"), tr("输入词头并按 Enter，或从词目索引中选择。", "Enter a headword and press Return, or choose one from the index."))
	}
	if d.results.Loading() {
		return empty(lucide.Search, tr("正在查词…", "Looking up…"), q.word)
	}
	if err := d.results.Err(); err != nil {
		return empty(lucide.CloudOff, tr("查询未完成", "Lookup failed"), err.Error())
	}
	items, _ := d.results.Get()
	if len(items) == 0 {
		return empty(lucide.SearchX, tr("没有找到匹配词条", "No matching entries"), q.word)
	}
	i := min(d.selected.Get(), len(items)-1)
	return d.entry(cx, items[i])
}

func (d *readingDesk) entry(cx *core.Scope, item entry) core.Node {
	tr := d.tr
	def := readDefinition(item.HTML)
	copied := core.NewSignal(cx, false)
	route := func(word string) { d.search(word) }
	scope := tr("本地词典", "Local dictionary")
	switch item.Visibility {
	case "public":
		scope = tr("公开词典", "Public dictionary")
	case "private":
		scope = tr("私有词典", "Private dictionary")
	}
	top := []core.Node{core.Icon{SVG: lucide.BookOpen(), Style: core.Style{FontSize: 16}}, core.Text{Content: item.Dictionary, Style: ui.TypeLabel, MaxLines: 1}, mutedText(cx, "· "+scope)}
	header := []core.Node{
		core.View{Style: rowStyle, Children: []core.Node{
			core.View{Style: rowStyle.Merge(core.Style{FlexGrow: 1, FlexShrink: 1, Wrap: core.Wrap}), Children: top},
			core.Show{When: cx.Env().HasWebView, Then: func(*core.Scope) core.Node {
				return core.Dynamic(func(*core.Scope) core.Node {
					name := tr("原版排版", "Original layout")
					if d.original.Get() {
						name = tr("阅读排版", "Reader layout")
					}
					return ui.Button{Text: name, Variant: ui.ButtonGhost, TestID: "layout", OnPress: func() { d.original.Set(!d.original.Get()) }}
				})
			}},
			core.Dynamic(func(*core.Scope) core.Node {
				icon, name := lucide.Copy, tr("复制释义", "Copy definition")
				if copied.Get() {
					icon, name = lucide.Check, tr("已复制", "Copied")
				}
				return ui.Button{Icon: icon, Label: name, Variant: ui.ButtonGhost, TestID: "copy", OnPress: func() {
					cx.SetClipboard(item.Word + " · " + item.Dictionary + "\n\n" + def.document.PlainText())
					copied.Set(true)
					cx.AfterFunc(1600*time.Millisecond, func() { copied.Set(false) })
				}}
			}),
		}},
		core.Text{Content: item.Word, Style: title("", 40).Style, Selectable: core.SelectableOn, TestID: "headword"},
		rule(cx),
	}
	body := []core.Node{
		ui.TextView{Document: def.document, Selectable: true, TestID: "definition", StyleOf: func(part textview.Part) core.Style {
			if part != textview.PartRoot {
				return core.Style{}
			}
			size := core.Points(d.prefs.size.Get())
			s := core.Style{FontSize: size, LineHeight: size * 1.55, Gap: 12}
			if d.prefs.typeface.Get() == "serif" {
				s.FontFamily = bookSerif
			}
			return s
		}, OnLinkPress: func(e textview.LinkPressEvent) {
			if w := linkedWord(e.URL); w != "" {
				route(w)
			}
		},
			RenderImage: func(cx *core.Scope, i textview.Image) core.Node {
				return attachmentView(cx, tr, d.source, item, attachment{"image", i.Src, i.Alt})
			},
			RenderCustom: func(cx *core.Scope, n textview.Custom) core.Node {
				if n.Name == "pronunciation" {
					return attachmentView(cx, tr, d.source, item, attachment{"audio", n.Attributes["src"], ""})
				}
				return nil
			}},
	}
	if len(def.related) > 0 {
		body = append(body, rule(cx), mutedText(cx, tr("关联词条", "Related entries")))
		var links []core.Node
		for i, w := range def.related {
			links = append(links, ui.Button{Text: w, Icon: lucide.ArrowUpRight, Variant: ui.ButtonOutline, TestID: fmt.Sprintf("related-%d", i), OnPress: func() { route(w) }})
		}
		body = append(body, core.View{Style: rowStyle.Merge(core.Style{Wrap: core.Wrap}), Children: links})
	}
	padding := func() core.Points {
		if cx.Env().Window().Width < 650 {
			return 16
		}
		return 24
	}
	return core.View{Style: paneStyle.Merge(core.Style{Gap: 0}), Children: []core.Node{
		core.View{StyleFunc: func() core.Style {
			return core.Style{Padding: core.Edges{Top: padding(), Left: padding(), Right: padding()}, Gap: 8}
		}, Children: header},
		core.Dynamic(func(cx *core.Scope) core.Node {
			if d.original.Get() {
				return originalView(cx, d, item)
			}
			return core.Scroll{Style: paneStyle, Children: []core.Node{core.View{StyleFunc: func() core.Style { return core.Style{Padding: core.All(padding()), Gap: 13} }, Children: body}}}
		}),
	}}
}
