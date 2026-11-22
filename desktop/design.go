package main

import (
	_ "embed"

	"github.com/nsgo-dev/nsgo/core"
	"github.com/nsgo-dev/nsgo/icons/lucide"
	"github.com/nsgo-dev/nsgo/ui"
)

//go:embed assets/owl.png
var owlIcon []byte

const bookSerif = "Georgia, Noto Serif CJK SC, Songti SC, serif"

type readingPrefs struct {
	appearance *core.Persisted[string]
	typeface   *core.Persisted[string]
	size       *core.Persisted[int]
}

func readingPreferences(cx *core.Scope) *readingPrefs {
	p := &readingPrefs{
		appearance: core.NewPersisted(cx, "appearance", "system", core.PersistOptions{}),
		typeface:   core.NewPersisted(cx, "typeface", "serif", core.PersistOptions{}),
		size:       core.NewPersisted(cx, "type-size", 17, core.PersistOptions{}),
	}
	cx.Effect(func() {
		appearance := core.AppearanceSystem
		switch p.appearance.Get() {
		case "light":
			appearance = core.AppearanceLight
		case "dark":
			appearance = core.AppearanceDark
		}
		cx.SetAppearance(appearance)
	})
	return p
}

func preferenceMenu(cx *core.Scope, tr label, p *readingPrefs) core.Node {
	return ui.Popover{
		Trigger: ui.Button{Icon: lucide.SlidersHorizontal, Variant: ui.ButtonGhost, Side: 34},
		Label:   tr("阅读设置", "Reading settings"), TestID: "reading-settings", Align: core.AlignEnd,
		Children: []core.Node{
			core.Text{Content: tr("阅读设置", "Reading settings"), Style: ui.TypeTitle},
			mutedText(cx, tr("外观", "Appearance")),
			ui.Select[string]{Value: p.appearance.Get, OnChange: func(s string) { _ = p.appearance.Set(s) }, Label: tr("外观", "Appearance"), TestID: "appearance",
				Options: []ui.Option[string]{{Value: "system", Label: tr("跟随系统", "System")}, {Value: "light", Label: tr("纸白", "Paper")}, {Value: "dark", Label: tr("深色", "Dark")}}},
			mutedText(cx, tr("正文字体", "Reading font")),
			ui.Select[string]{Value: p.typeface.Get, OnChange: func(s string) { _ = p.typeface.Set(s) }, Label: tr("正文字体", "Reading font"),
				Options: []ui.Option[string]{{Value: "serif", Label: tr("衬线", "Serif")}, {Value: "sans", Label: tr("无衬线", "Sans serif")}}},
			mutedText(cx, tr("文字大小", "Text size")),
			ui.Select[int]{Value: p.size.Get, OnChange: func(s int) { _ = p.size.Set(s) }, Label: tr("文字大小", "Text size"),
				Options: []ui.Option[int]{{Value: 15, Label: tr("紧凑", "Small")}, {Value: 17, Label: tr("标准", "Standard")}, {Value: 20, Label: tr("大字", "Large")}}},
		},
	}
}

func pageStyle(cx *core.Scope, padding core.Points) core.Style {
	p := cx.Palette()
	return core.Style{Background: p.Surface, Color: p.Text, BorderWidth: 1, BorderColor: p.Border,
		CornerRadius: 12, BorderCurve: core.BorderCurveContinuous, Padding: core.All(padding), Gap: 14,
		ShadowColor: core.RGBA(0x322718, .06), ShadowOffset: core.Point{Y: 1}, ShadowRadius: 1.5}
}

func mutedText(cx *core.Scope, s string) core.Text {
	return core.Text{Content: s, Style: ui.TypeCaption, StyleFunc: func() core.Style { return core.Style{Color: cx.Palette().TextMuted} }}
}

func title(text string, size core.Points) core.Text {
	return core.Text{Content: text, Style: ui.TypeHeading.Merge(core.Style{FontFamily: bookSerif, FontSize: size, LineHeight: size * 1.23})}
}

func rule(cx *core.Scope) core.Node {
	return core.View{StyleFunc: func() core.Style { return core.Style{Height: core.Pt(1), Background: cx.Palette().Border} }}
}

func sectionTitle(cx *core.Scope, glyph func() *core.SVG, text string) core.Node {
	return core.View{Style: rowStyle, Children: []core.Node{
		core.Icon{SVG: glyph(), StyleFunc: func() core.Style { return core.Style{FontSize: 15, Color: cx.Palette().Accent} }},
		core.Text{Content: text, Style: ui.TypeLabel},
	}}
}

func setupPage(cx *core.Scope, glyph func() *core.SVG, heading, detail string, controls ...core.Node) core.Node {
	contents := []core.Node{
		core.Icon{SVG: glyph(), StyleFunc: func() core.Style {
			return core.Style{Width: core.Pt(36), Height: core.Pt(36), AlignSelf: core.AlignStart, Color: cx.Palette().Accent}
		}},
		title(heading, 31),
		core.Text{Content: detail, Style: core.Style{FontSize: 15, LineHeight: 23}, StyleFunc: func() core.Style { return core.Style{Color: cx.Palette().TextMuted} }},
		rule(cx),
	}
	contents = append(contents, controls...)
	return core.Scroll{Style: paneStyle, Children: []core.Node{core.View{StyleFunc: func() core.Style {
		s := pageStyle(cx, 28)
		s.MaxWidth, s.Width, s.AlignSelf = core.Pt(590), core.Pct(100), core.AlignCenter
		s.Margin = core.Edges{Top: 20, Bottom: 16}
		return s
	}, Children: contents}}}
}
