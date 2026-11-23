package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func sidebarCSSBlock(t *testing.T, css, selector string) string {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(selector) + `\s*\{([^}]*)\}`)
	m := pattern.FindStringSubmatch(css)
	if m == nil {
		t.Fatalf("missing CSS rule %q", selector)
	}
	return m[1]
}

func sidebarCSSValue(body, property string) string {
	pattern := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(property) + `\s*:\s*([^;]+);`)
	m := pattern.FindStringSubmatch(body)
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[1])
}

func TestPlatformSidebarTextContrast(t *testing.T) {
	data, err := os.ReadFile("app/navigation.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)

	cases := []struct {
		selector string
		property string
		want     string
	}{
		{".platform-sidebar", "--muted", "var(--hui-secondary)"},
		{".platform-sidebar", "--muted-2", "var(--hui-muted)"},
		{".platform-sidebar", "--blue-soft", "var(--hui-accent-soft)"},
		{".platform-sidebar", "--text", "var(--hui-text)"},
		{`html[data-theme="dark"] .platform-sidebar`, "--muted", "var(--hui-secondary)"},
		{`html[data-theme="dark"] .platform-sidebar`, "--muted-2", "var(--hui-muted)"},
		{`html[data-theme="dark"] .platform-sidebar`, "--blue-soft", "var(--hui-accent-soft)"},
		{".management-navigation > p", "color", "var(--muted-2)"},
		{".platform-sidebar .nav", "color", "var(--muted)"},
		{"html[data-theme] .platform-sidebar .nav.active", "background", "var(--blue-soft)"},
		{"html[data-theme] .platform-sidebar .nav.active", "color", "var(--text)"},
		{"html[data-theme] .platform-sidebar .nav.active", "font-weight", "600"},
	}
	for _, c := range cases {
		body := sidebarCSSBlock(t, css, c.selector)
		if got := sidebarCSSValue(body, c.property); got != c.want {
			t.Errorf("%s { %s } = %q, want %q", c.selector, c.property, got, c.want)
		}
	}
}

func TestChatbotSidebarTextContrast(t *testing.T) {
	data, err := os.ReadFile("../chatbot/src/navigation.css")
	if err != nil {
		t.Fatal(err)
	}
	css := string(data)

	cases := []struct {
		selector string
		property string
		want     string
	}{
		{".chat-shell .conversation-sidebar", "--muted", "var(--hui-secondary)"},
		{".chat-shell .conversation-sidebar", "--navigation-secondary", "var(--hui-muted)"},
		{`[data-theme="dark"] .chat-shell .conversation-sidebar`, "--muted", "var(--hui-secondary)"},
		{`[data-theme="dark"] .chat-shell .conversation-sidebar`, "--navigation-secondary", "var(--hui-muted)"},
		{".chat-shell .section-label", "color", "var(--muted)"},
		{".chat-shell .sidebar-section-heading", "color", "var(--muted)"},
		{".chat-shell .session-row.active .session-link", "font-weight", "600"},
	}
	for _, c := range cases {
		body := sidebarCSSBlock(t, css, c.selector)
		if got := sidebarCSSValue(body, c.property); got != c.want {
			t.Errorf("%s { %s } = %q, want %q", c.selector, c.property, got, c.want)
		}
	}

	ink, err := os.ReadFile("app/hover-ink.css")
	if err != nil {
		t.Fatal(err)
	}
	body := sidebarCSSBlock(t, string(ink), ".chat-shell .session-row.active")
	if got := sidebarCSSValue(body, "background"); got != "var(--hui-accent-soft)" {
		t.Errorf(".chat-shell .session-row.active { background } = %q, want %q", got, "var(--hui-accent-soft)")
	}
}
