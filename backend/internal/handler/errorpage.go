package handler

import (
	"bytes"
	_ "embed"
	"html/template"

	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

//go:embed templates/error.html
var errorPageHTML string

var errorPageTmpl = template.Must(template.New("error").Parse(errorPageHTML))

type errorPageData struct {
	Title   string
	Message string
	HomeURL string
}

// renderErrorPage writes the redirect server's HTML error page.
func renderErrorPage(c *gin.Context, status int, title, message, homeURL string) {
	var buf bytes.Buffer
	if err := errorPageTmpl.Execute(&buf, errorPageData{Title: title, Message: message, HomeURL: homeURL}); err != nil {
		logger.Error(c.Request.Context(), "redirect: failed to render error page", zap.Error(err))
		c.String(status, title)
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Data(status, "text/html; charset=utf-8", buf.Bytes())
}
