package message

import "github.com/microcosm-cc/bluemonday"

var defaultPolicy = buildSanitizePolicy()

func buildSanitizePolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "b", "i", "u", "strong", "em", "ul", "ol", "li",
		"h1", "h2", "h3", "h4", "h5", "h6", "blockquote", "pre", "code",
		"table", "thead", "tbody", "tr", "th", "td", "a", "span", "div")
	p.AllowAttrs("href").OnElements("a")
	p.RequireParseableURLs(true)
	p.AllowURLSchemes("http", "https", "mailto")
	p.AllowAttrs("class").Globally()
	return p
}

func SanitizeHTML(html string, loadImages bool) string {
	if html == "" {
		return ""
	}
	if loadImages {
		p := bluemonday.NewPolicy()
		p.AllowElementsMatching(nil)
		p.AllowAttrs("src", "alt", "width", "height").OnElements("img")
		html = p.Sanitize(html)
	}
	return defaultPolicy.Sanitize(html)
}
