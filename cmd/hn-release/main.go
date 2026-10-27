// hn-release packages a committed Hubnot revision locally; it never publishes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"hubnot/internal/release"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "hn-release: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("hn-release", flag.ContinueOnError)
	flags.SetOutput(out)
	var options release.Options
	var targets, variants string
	flags.StringVar(&options.Repository, "source", ".", "source Git working tree")
	flags.StringVar(&options.Revision, "rev", "", "explicit committed source revision (required)")
	flags.StringVar(&options.Version, "version", "", "X.Y.Z[-prerelease], without leading v (required)")
	flags.StringVar(&options.Output, "out", "", "new output directory; must not exist (required)")
	flags.StringVar(&targets, "targets", "darwin/amd64,darwin/arm64,linux/amd64,linux/arm64", "comma-separated target subset")
	flags.StringVar(&variants, "variants", "standard,native-only", "comma-separated variant subset")
	flags.Usage = func() {
		fmt.Fprintln(out, "Usage: hn-release --rev REV --version VERSION --out DIRECTORY [options]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if targets == "" || variants == "" {
		return errors.New("targets and variants must be nonempty")
	}
	options.Targets = strings.Split(targets, ",")
	options.Variants = strings.Split(variants, ",")
	if err := release.Package(ctx, options); err != nil {
		return err
	}
	fmt.Fprintf(out, "Release assets written to %s\n", options.Output)
	return nil
}
