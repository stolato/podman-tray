// Package i18n translates the app's texts. They are written in English in the code, which
// is also the fallback; pt.go maps them to Portuguese. The language follows the system's
// (lang_*.go), and PODMAN_TRAY_LANG (e.g. "en", "pt-BR") overrides it.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

var catalog = pick(language())

func language() string {
	if l := os.Getenv("PODMAN_TRAY_LANG"); l != "" {
		return l
	}
	return systemLanguage()
}

// pick returns the translations for a language tag ("pt-BR", "pt_BR.UTF-8"...), or nil
// for English and anything without translations.
func pick(lang string) map[string]string {
	lang = strings.ToLower(lang)
	if lang == "pt" || strings.HasPrefix(lang, "pt-") || strings.HasPrefix(lang, "pt_") {
		return pt
	}
	return nil
}

// T translates an English text.
func T(en string) string {
	if s, ok := catalog[en]; ok {
		return s
	}
	return en
}

// F translates an English format string, then formats it like fmt.Sprintf.
func F(en string, args ...any) string {
	return fmt.Sprintf(T(en), args...)
}

// envLanguage reads the POSIX locale variables, in order of precedence.
func envLanguage() string {
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if l := os.Getenv(v); l != "" && l != "C" && l != "POSIX" && !strings.HasPrefix(l, "C.") {
			return l
		}
	}
	return ""
}
