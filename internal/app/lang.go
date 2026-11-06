package app

import (
	"os"
	"strings"

	"radlog/internal/i18n"
	"radlog/internal/pdf"
)

// uiLang é o idioma em que o app escreve para você: relatórios, bandeja,
// título da janela e linhas do registro técnico. A tela informa o idioma já
// resolvido (SetUILang); antes disso vale a escolha das Configurações, e no
// "automático" o idioma do sistema.
func (a *App) uiLang() string {
	a.mu.Lock()
	l, choice := a.lang, a.settings.Lang
	a.mu.Unlock()
	if l != "" {
		return l
	}
	return i18n.Lang(choice, sysLocale())
}

func sysLocale() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// SetUILang recebe da tela o idioma em uso ("pt" ou "en").
func (a *App) SetUILang(lang string) {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if lang != "en" {
		lang = "pt"
	}
	a.mu.Lock()
	changed := a.lang != lang
	a.lang = lang
	a.mu.Unlock()
	if changed {
		a.trayRelabel()
	}
}

// T traduz um texto do app para o idioma da interface.
func (a *App) T(s string) string { return i18n.T(a.uiLang(), s) }

// inLang gera um PDF no idioma da interface (em português, sem tradução).
func (a *App) inLang(render func() []byte) []byte {
	lang := a.uiLang()
	if lang == "pt" {
		return render()
	}
	return pdf.WithTranslator(func(s string) string { return i18n.T(lang, s) }, render)
}
