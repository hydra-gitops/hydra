package highlight

import (
	"bytes"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

const (
	formatterTerminal256 = "terminal256"
	styleVim             = "vim"
)

// helmTemplateYAMLLexer highlights Helm-style charts: Go text templates (`{{ … }}`) combined with
// YAML in the gaps, via chroma.DelegatingLexer(yaml, go-text-template).
var helmTemplateYAMLLexer = chroma.DelegatingLexer(lexers.Get("yaml"), lexers.GoTextTemplate)

// GoTemplateTerminal256 applies Chroma syntax highlighting for Go/Helm-style templates when color
// is enabled: template actions use the go-text-template lexer; stretches outside `{{ … }}` are
// re-lexed as YAML (same approach as chroma’s go-html-template).
func GoTemplateTerminal256(color types.Color, source string) (string, error) {
	if !color {
		return source, nil
	}
	f := formatters.Get(formatterTerminal256)
	if f == nil {
		f = formatters.Fallback
	}
	s := styles.Get(styleVim)
	if s == nil {
		s = styles.Fallback
	}
	it, err := helmTemplateYAMLLexer.Tokenise(nil, source)
	if err != nil {
		return source, log.CreateError(errors.ErrChromaHighlightFailed, "syntax highlighting failed: {err}", log.Err(err))
	}
	var buf bytes.Buffer
	if err := f.Format(&buf, s, it); err != nil {
		return source, log.CreateError(errors.ErrChromaHighlightFailed, "syntax highlighting failed: {err}", log.Err(err))
	}
	return buf.String(), nil
}
