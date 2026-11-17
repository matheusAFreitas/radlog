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
// Com a taxa da tela em mrem/h, os textos com número em µSv viram mrem
// (2.84.0, unidades.go); os gráficos do PDF continuam em µSv/h.
func (a *App) inLang(render func() []byte) []byte {
	lang := a.uiLang()
	a.mu.Lock()
	mrem := a.settings.RateUnit == rateMrem
	iso := a.settings.DateFormat == dateISO
	a.mu.Unlock()
	if lang == "pt" && !mrem && !iso {
		return render()
	}
	sep := ","
	if lang == "en" {
		sep = "."
	}
	return pdf.WithTranslator(func(s string) string {
		if lang != "pt" {
			s = i18n.T(lang, s)
		}
		if mrem {
			s = remText(s, sep)
		}
		if iso {
			s = isoText(s)
		}
		return s
	}, render)
}
