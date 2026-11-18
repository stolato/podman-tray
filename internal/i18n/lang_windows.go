package i18n

import "golang.org/x/sys/windows"

// systemLanguage is the first of the user's display languages, e.g. "pt-BR".
func systemLanguage() string {
	langs, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	if err != nil || len(langs) == 0 {
		return envLanguage()
	}
	return langs[0]
}
