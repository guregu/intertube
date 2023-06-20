package web

import (
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/kardianos/osext"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

var translations *i18n.Bundle
var defaultLocalizer *i18n.Localizer

func loadTranslations() {
	here, err := osext.ExecutableFolder()
	if err != nil {
		panic(err)
	}
	translations = i18n.NewBundle(language.English)
	translations.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	// TODO: walk text directory
	translations.MustLoadMessageFile(filepath.Join(here, "assets", "text", "en.toml"))
	// translations.MustLoadMessageFile(filepath.Join(here, "assets", "text", "ja.toml"))
	defaultLocalizer = i18n.NewLocalizer(translations, language.Japanese.String())
}

func translateFunc(localizer *i18n.Localizer) interface{} {
	return func(id string, args ...interface{}) string {
		var data map[string]interface{}
		if len(args) > 0 {
			data = make(map[string]interface{}, len(args))
			for n, iface := range args {
				data["v"+strconv.Itoa(n)] = iface
			}
		}
		cfg := &i18n.LocalizeConfig{
			MessageID:    id,
			TemplateData: data,
		}
		str, err := localizer.Localize(cfg)
		if err != nil {
			if str, err = defaultLocalizer.Localize(cfg); err == nil {
				return str
			}
			return "{TL err: " + err.Error() + "}"
		}
		return str
	}
}

func translateCountFunc(localizer *i18n.Localizer) interface{} {
	return func(id string, ct int, args ...interface{}) string {
		data := make(map[string]interface{}, len(args)+1)
		if len(args) > 0 {
			for n, iface := range args {
				data["v"+strconv.Itoa(n)] = iface
			}
		}
		data["ct"] = ct
		cfg := &i18n.LocalizeConfig{
			MessageID:    id,
			TemplateData: data,
			PluralCount:  ct,
		}
		str, err := localizer.Localize(cfg)
		if err != nil {
			if str, err = defaultLocalizer.Localize(cfg); err == nil {
				return str
			}
			return "{TL err: " + err.Error() + "}"
		}
		return str
	}
}
