package picker

import (
	"errors"
	"fmt"

	"github.com/ktr0731/go-fuzzyfinder"

	"github.com/tryoverflow/ssmhop/internal/target"
)

var ErrAborted = errors.New("selection cancelled")

type Options struct {
	Noun    string
	Region  string
	Profile string
	Query   string
	NoFzf   bool
}

func (o Options) header() string {
	h := fmt.Sprintf("region (%s)", o.Region)
	if o.Profile != "" {
		h += fmt.Sprintf("  profile (%s)", o.Profile)
	}
	return h + "   [use arrows to move, type to filter]"
}

func (o Options) prompt() string {
	noun := o.Noun
	if noun == "" {
		noun = "target"
	}
	article := "a"
	switch noun[0] {
	case 'a', 'e', 'i', 'o', 'u':
		article = "an"
	}
	return fmt.Sprintf("Choose %s %s in AWS: ", article, noun)
}

func Pick(items []target.Target, opts Options) (target.Target, error) {
	if len(items) == 0 {
		return target.Target{}, errors.New("no targets found")
	}
	if len(items) == 1 {
		return items[0], nil
	}

	if !opts.NoFzf {
		idx, err := fzfPick(items, opts)
		if err == nil {
			return items[idx], nil
		}
		if !errors.Is(err, errFzfUnusable) {
			return target.Target{}, err
		}
	}

	return embeddedPick(items, opts)
}

func embeddedPick(items []target.Target, opts Options) (target.Target, error) {
	idx, err := fuzzyfinder.Find(items,
		func(i int) string { return items[i].Label() },
		fuzzyfinder.WithPromptString(opts.prompt()),
		fuzzyfinder.WithQuery(opts.Query),
		fuzzyfinder.WithSelectOne(),
		fuzzyfinder.WithHeader(opts.header()),
	)
	if err != nil {
		if errors.Is(err, fuzzyfinder.ErrAbort) {
			return target.Target{}, ErrAborted
		}
		return target.Target{}, fmt.Errorf("select %s: %w", opts.Noun, err)
	}
	return items[idx], nil
}
