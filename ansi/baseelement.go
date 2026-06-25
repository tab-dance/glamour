package ansi

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"text/template"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// BaseElement renders a styled primitive element.
type BaseElement struct {
	Token  string
	Prefix string
	Suffix string
	Style  StylePrimitive
}

var (
	templateCache   = make(map[string]*template.Template)
	templateCacheMu sync.RWMutex

	upperCaser = cases.Upper(language.English)
	lowerCaser = cases.Lower(language.English)
	titleCaser = cases.Title(language.English)
)

func formatToken(format string, token string) (string, error) {
	templateCacheMu.RLock()
	tmpl, ok := templateCache[format]
	templateCacheMu.RUnlock()

	if !ok {
		var err error
		tmpl, err = template.New(format).Funcs(TemplateFuncMap).Parse(format)
		if err != nil {
			return "", fmt.Errorf("glamour: error parsing template: %w", err)
		}
		templateCacheMu.Lock()
		templateCache[format] = tmpl
		templateCacheMu.Unlock()
	}

	var b bytes.Buffer
	v := map[string]any{"text": token}
	if err := tmpl.Execute(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

func renderText(w io.Writer, rules StylePrimitive, s string) (int, error) { //nolint:unparam
	if len(s) == 0 {
		return 0, nil
	}

	// XXX: We're using [ansi.Style] instead of [lipgloss.Style] because
	// Lip Gloss has a weird bug where it adds spaces when rendering joined
	// strings. Needs further investigation.
	style := ansi.Style{}
	if rules.Upper != nil && *rules.Upper {
		s = upperCaser.String(s)
	}
	if rules.Lower != nil && *rules.Lower {
		s = lowerCaser.String(s)
	}
	if rules.Title != nil && *rules.Title {
		s = titleCaser.String(s)
	}
	if rules.Color != nil {
		style = style.ForegroundColor(lipgloss.Color(*rules.Color))
	}
	if rules.BackgroundColor != nil {
		style = style.BackgroundColor(lipgloss.Color(*rules.BackgroundColor))
	}
	if rules.Underline != nil && *rules.Underline {
		style = style.Underline(true)
	}
	if rules.Bold != nil && *rules.Bold {
		style = style.Bold()
	}
	if rules.Italic != nil && *rules.Italic {
		style = style.Italic(true)
	}
	if rules.CrossedOut != nil && *rules.CrossedOut {
		style = style.Strikethrough(true)
	}
	if rules.Inverse != nil && *rules.Inverse {
		style = style.Reverse(true)
	}
	if rules.Blink != nil && *rules.Blink {
		style = style.Blink(true)
	}

	n, err := io.WriteString(w, style.Styled(s))
	if err != nil {
		return n, fmt.Errorf("glamour: error writing to writer: %w", err)
	}

	return n, nil
}

// StyleOverrideRender renders a BaseElement with an overridden style.
func (e *BaseElement) StyleOverrideRender(w io.Writer, ctx RenderContext, style StylePrimitive) error {
	bs := ctx.blockStack
	st1 := cascadeStylePrimitives(bs.Current().Style.StylePrimitive, style)
	st2 := cascadeStylePrimitives(bs.With(e.Style), style)

	return e.doRender(w, st1, st2)
}

// Render renders a BaseElement.
func (e *BaseElement) Render(w io.Writer, ctx RenderContext) error {
	bs := ctx.blockStack
	st1 := bs.Current().Style.StylePrimitive
	st2 := bs.With(e.Style)
	return e.doRender(w, st1, st2)
}

func (e *BaseElement) doRender(w io.Writer, st1, st2 StylePrimitive) error {
	_, _ = renderText(w, st1, e.Prefix)
	defer func() {
		_, _ = renderText(w, st1, e.Suffix)
	}()

	// render unstyled prefix/suffix
	_, _ = renderText(w, st1, st2.BlockPrefix)
	defer func() {
		_, _ = renderText(w, st1, st2.BlockSuffix)
	}()

	// render styled prefix/suffix
	_, _ = renderText(w, st2, st2.Prefix)
	defer func() {
		_, _ = renderText(w, st2, st2.Suffix)
	}()

	s := e.Token
	if len(st2.Format) > 0 {
		var err error
		s, err = formatToken(st2.Format, s)
		if err != nil {
			return err
		}
	}
	if strings.Contains(s, "\\") {
		s = escapeReplacer.Replace(s)
	}
	_, _ = renderText(w, st2, s)
	return nil
}

// https://www.markdownguide.org/basic-syntax/#characters-you-can-escape
var escapeReplacer = strings.NewReplacer(
	"\\\\", "\\",
	"\\`", "`",
	"\\*", "*",
	"\\_", "_",
	"\\{", "{",
	"\\}", "}",
	"\\[", "[",
	"\\]", "]",
	"\\<", "<",
	"\\>", ">",
	"\\(", "(",
	"\\)", ")",
	"\\#", "#",
	"\\+", "+",
	"\\-", "-",
	"\\.", ".",
	"\\!", "!",
	"\\|", "|",
)
