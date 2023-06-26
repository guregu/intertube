package web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/fsnotify/fsnotify"
	"github.com/kardianos/osext"

	"github.com/guregu/intertube/storage"
	"github.com/guregu/intertube/tube"
)

var templates *template.Template

func getTemplate(ctx context.Context, name string) *template.Template {
	t := templates.Lookup(name + ".gohtml")
	if t == nil {
		panic("no template: " + name)
	}

	t, err := t.Clone()
	if err != nil {
		panic(err)
	}
	t.Funcs(templateFuncs(ctx))
	return t
}

func templateFuncs(ctx context.Context) template.FuncMap {
	m := make(template.FuncMap)

	// TODO: use jst/account settings and overwrite time stuff

	localizer := localizerFrom(ctx)
	lang := languageFrom(ctx)
	user, loggedIn := userFrom(ctx)

	m["render"] = renderFunc(ctx)
	m["stylesheet"] = renderCSSFunc(ctx, user.Theme)
	m["opts"] = func() tube.DisplayOptions { return user.Display }
	m["tr"] = translateFunc(localizer)
	m["tc"] = translateCountFunc(localizer)
	m["lang"] = func() string { return lang }
	m["path"] = func() string { return pathFrom(ctx) }
	m["loggedin"] = func() bool { return loggedIn }

	return m
}

func parseTemplates() *template.Template {
	here, err := osext.ExecutableFolder()
	if err != nil {
		panic(err)
	}
	glob := filepath.Join(here, "assets", "templates", "*.gohtml")

	t := template.New("root").Funcs(template.FuncMap{
		"render":     renderFunc(context.Background()),
		"stylesheet": renderCSSFunc(context.Background(), "default"),
		"opts":       func() tube.DisplayOptions { return tube.DisplayOptions{} },

		"timestamp": func(t time.Time) template.HTML {
			dateFmt := "2006-01-02 15:04"
			rfc := t.Format(time.RFC3339)
			return template.HTML(
				fmt.Sprintf(`<time datetime="%s">%s</time>`, rfc, t.Format(dateFmt)))
		},
		"date": func(t time.Time) template.HTML {
			dateFmt := "2006-01-02"
			rfc := t.Format(time.RFC3339)
			return template.HTML(
				fmt.Sprintf(`<time datetime="%s">%s</time>`, rfc, t.Format(dateFmt)))
		},
		"shortdate": func(t time.Time) string {
			layout := "01-02"
			now := time.Now().UTC()
			if t.Year() != now.Year() && now.Sub(t) >= 4*30*24*time.Hour {
				layout = "2006-01-02"
			}
			return t.Format(layout)
		},
		"days": func(d time.Duration) string {
			days := d.Round(24*time.Hour).Hours() / 24
			return fmt.Sprintf("%g", days)
		},
		"subtract": func(a, b int) int {
			return a - b
		},
		"inc": func(a int) int {
			return a + 1
		},
		"add": func(a int, b ...int) int {
			for _, n := range b {
				a += n
			}
			return a
		},
		"pctof": func(n int64, pct float64) int {
			return int(float64(n) * pct)
		},
		"concat": func(strs ...string) string {
			return strings.Join(strs, "")
		},
		"bespace": func(v []string) string {
			return strings.Join(v, " ")
		},
		"filesize": func(size int64) string {
			return humanize.Bytes(uint64(size))
		},
		"bytesize": func(size int64) string {
			str := humanize.IBytes(uint64(size))
			return strings.ReplaceAll(str, "iB", "B")
		},
		"currency": formatCurrency,

		"tr":       translateFunc(defaultLocalizer),
		"tc":       translateCountFunc(defaultLocalizer),
		"lang":     func() string { return "en" },
		"path":     func() string { return "" },
		"loggedin": func() bool { return false },

		"cdn": func(key string) (string, error) {
			// return attachmentHost + href
			return storage.FilesBucket.PresignGet(key)
		},
		"sign": func(href string) (string, error) {
			return signURL(href)
		},

		"blankzero": func(i int) string {
			if i == 0 {
				return ""
			}
			return strconv.Itoa(i)
		},
	})

	t = template.Must(t.ParseGlob(glob))

	// if DebugMode {
	// 	for _, tt := range t.Templates() {
	// 		fmt.Println("Template:", tt.Name())
	// 	}
	// }

	return t
}

func renderFunc(ctx context.Context) func(string, interface{}) (template.HTML, error) {
	return func(name string, data interface{}) (template.HTML, error) {
		target := getTemplate(ctx, name)
		if target == nil {
			return "", fmt.Errorf("render: missing template: %s", name)
		}
		var buf bytes.Buffer
		err := target.Execute(&buf, data)
		if err != nil {
			fmt.Println("ERR!!", err)
			return "", err
		}
		return template.HTML(buf.String()), nil
	}
}

func renderCSSFunc(ctx context.Context, active string) func(string, interface{}) (template.CSS, error) {
	if active == "" {
		active = "default"
	}
	return func(name string, data interface{}) (template.CSS, error) {
		if name == "@" {
			name = active
		}
		name = "_style-" + name
		target := getTemplate(ctx, name)
		if target == nil {
			return "", fmt.Errorf("render: missing template: %s", name)
		}
		var buf bytes.Buffer
		err := target.Execute(&buf, data)
		if err != nil {
			fmt.Println("ERR!!", err)
			return "", err
		}
		return template.CSS(buf.String()), nil
	}
}

// hot reload for dev
// this is racy but i don't care. lol
func WatchFiles() func() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	go func() {
		for {
			select {
			case ev := <-watcher.Events:
				log.Println("watch event:", ev)
				switch filepath.Ext(ev.Name) {
				case ".gohtml":
					log.Println("Reloading templates...", filepath.Base(ev.Name))
					templates = parseTemplates()
				case ".toml":
					log.Println("Reloading translations...", filepath.Base(ev.Name))
					loadTranslations()
				}
			case err := <-watcher.Errors:
				log.Println("watch error:", err)
			}
		}
	}()

	here, err := osext.ExecutableFolder()
	if err != nil {
		panic(err)
	}
	if err := watcher.Add(filepath.Join(here, "assets", "templates")); err != nil {
		panic(err)
	}
	if err := watcher.Add(filepath.Join(here, "assets", "text")); err != nil {
		panic(err)
	}
	return watcher.Close
}
