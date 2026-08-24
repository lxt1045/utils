package atlas

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/lxt1045/atlas-cmd-internal/cmdapi"
	"github.com/lxt1045/atlas-cmd-internal/cmdext"
	"github.com/lxt1045/atlas-cmd-internal/cmdlog"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlclient"
	"github.com/lxt1045/errors"
)

// --format '{{ sql . }}' : 一行 sql ;
// --format '{{ sql . \"  \" }}' : 转成多行，方便查看 ;
// atlas schema diff --from "mysql://root:password@127.0.0.1:3306/dji1" --to "file://e:/test/atlas/test.sql" --format '{{ sql . \"  \" }}' --dev-url "mysql://root:password@127.0.0.1:3306/atlas_dev"
// atlas schema diff --from "mysql://root:password@127.0.0.1:3306/dji1" --to "file://e:/test/atlas/test.sql" --format '{{ sql . }}' --dev-url "mysql://root:password@127.0.0.1:3306/atlas_dev"
func SchemaDiffRun(ctx context.Context, fromURL, toURL, schemas, exclude []string, formatTo, devURL string) (err error) {
	var (
		c *sqlclient.Client
	)
	c, err = sqlclient.Open(ctx, devURL)
	if err != nil {
		return err
	}
	defer c.Close()

	from, err := stateReader(ctx, &stateReaderConfig{
		urls:    fromURL,
		dev:     c,
		vars:    nil,
		schemas: schemas,
		exclude: exclude,
	})
	if err != nil {
		return err
	}
	defer from.Close()
	to, err := stateReader(ctx, &stateReaderConfig{
		urls:    toURL,
		dev:     c,
		vars:    nil,
		schemas: schemas,
		exclude: exclude,
	})
	if err != nil {
		return err
	}
	defer to.Close()
	if c == nil {
		// If not both states are provided by a database connection, the call to state-reader would have returned
		// an error already. If we land in this case, we can assume both states are database connections.
		c = to.Closer.(*sqlclient.Client)
	}
	format := cmdlog.SchemaDiffTemplate
	if v := formatTo; v != "" {
		if format, err = template.New("format").Funcs(cmdlog.SchemaDiffFuncs).Parse(v); err != nil {
			return fmt.Errorf("parse log format: %w", err)
		}
	}
	diff, err := computeDiff(ctx, c, from, to, diffOptions()...)
	if err != nil {
		return err
	}

	schemaDiff := cmdlog.NewSchemaDiff(ctx, c, diff.from, diff.to, diff.changes)
	return format.Execute(os.Stdout, schemaDiff)
}

// selectScheme validates the scheme of the provided to urls and returns the selected
// url scheme. Currently, all URLs must be of the same scheme, and only multiple
// "file://" URLs are allowed.
func selectScheme(urls []string) (string, error) {
	var scheme string
	if len(urls) == 0 {
		return "", errors.New("at least one url is required")
	}
	for _, u := range urls {
		parts := strings.SplitN(u, "://", 2)
		switch current := parts[0]; {
		case len(parts) == 1:
			ex := filepath.Ext(u)
			switch f, err := os.Stat(u); {
			case err != nil:
			case f.IsDir(), ex == cmdext.FileTypeSQL, ex == cmdext.FileTypeHCL:
				return "", fmt.Errorf("missing scheme. Did you mean file://%s?", u)
			}
			return "", errors.New("missing scheme. See: https://atlasgo.io/url")
		case scheme == "":
			scheme = current
		case scheme != current:
			return "", fmt.Errorf("got mixed --to url schemes: %q and %q, the desired state must be provided from a single kind of source", scheme, current)
		case current != cmdext.SchemaTypeFile:
			return "", fmt.Errorf("got multiple --to urls of scheme %q, only multiple 'file://' urls are supported", current)
		}
	}
	return scheme, nil
}

// stateReaderConfig is given to stateReader.
type stateReaderConfig struct {
	urls        []string          // urls to create a migrate.StateReader from; https://atlasgo.io/concepts/url
	client, dev *sqlclient.Client // database connections, while dev is considered a dev database, client is not
	schemas     []string          // schemas to work on
	exclude     []string          // exclude flag values
	include     []string          // include flag values
	withPos     bool              // indicate if schema.Pos should be loaded.
	vars        cmdapi.Vars
}

func stateReader(ctx context.Context, config *stateReaderConfig) (*cmdext.StateReadCloser, error) {
	if config.vars == nil {
		config.vars = make(cmdapi.Vars)
	}
	excfg, err := func() (conf *cmdext.StateReaderConfig, err error) {
		parsed := make([]*url.URL, len(config.urls))
		for i, u := range config.urls {
			if parsed[i], err = sqlclient.ParseURL(u); err != nil {
				return nil, err
			}
		}
		return &cmdext.StateReaderConfig{
			URLs:    parsed,
			Client:  config.client,
			Dev:     config.dev,
			Schemas: config.schemas,
			Exclude: config.exclude,
			Include: config.include,
			WithPos: config.withPos,
			Vars:    config.vars,
		}, nil
	}()
	if err != nil {
		return nil, err
	}

	scheme, err := selectScheme(config.urls)
	if err != nil {
		return nil, err
	}
	switch scheme {
	// "file" scheme is valid for both migration directory and HCL paths.
	case cmdext.SchemaTypeFile:
		switch ext, err := cmdext.FilesExt(excfg.URLs); {
		case err != nil:
			return nil, err
		case ext == cmdext.FileTypeHCL:
			return cmdext.StateReaderHCL(ctx, excfg)
		case ext == cmdext.FileTypeSQL:
			return cmdext.StateReaderSQL(ctx, excfg)
		default:
			panic("unreachable") // checked by filesExt.
		}
	case cmdext.SchemaTypeAtlas, "env":
		return nil, cmdext.UnsupportedErr("atlas remote state")
	default:
		// In case there is an external state-loader registered with this scheme.
		if l, ok := cmdext.States.Loader(scheme); ok {
			rc, err := l.LoadState(ctx, excfg)
			if err != nil {
				return nil, err
			}
			return rc, nil
		}
		// All other schemes are database (or docker) connections.
		c, err := sqlclient.Open(ctx, config.urls[0]) // call to selectScheme already checks for len > 0
		if err != nil {
			return nil, err
		}
		var sr migrate.StateReader
		switch c.URL.Schema {
		case "":
			sr = migrate.RealmConn(c.Driver, &schema.InspectRealmOption{
				Schemas: config.schemas,
				Exclude: config.exclude,
				Include: config.include,
			})
		default:
			sr = migrate.SchemaConn(c.Driver, c.URL.Schema, &schema.InspectOptions{
				Exclude: config.exclude,
				Include: config.include,
			})
		}
		return &cmdext.StateReadCloser{
			StateReader: sr,
			Closer:      c,
			Schema:      c.URL.Schema,
		}, nil
	}
}

func diffOptions() (opts []schema.DiffOption) {
	var (
		changes schema.Changes
	)
	for _, c := range []schema.Change{
		&schema.AddSchema{}, &schema.DropSchema{}, &schema.ModifySchema{},
		&schema.AddTable{}, &schema.DropTable{}, &schema.ModifyTable{}, &schema.RenameTable{},
		&schema.AddColumn{}, &schema.DropColumn{}, &schema.ModifyColumn{}, &schema.AddIndex{},
		&schema.DropIndex{}, &schema.ModifyIndex{}, &schema.AddForeignKey{}, &schema.DropForeignKey{},
		&schema.ModifyForeignKey{}, &schema.RenameConstraint{},
	} {
		changes = append(changes, c)
	}
	if len(changes) > 0 {
		// opts = append(opts, schema.DiffSkipChanges(changes...))
	}
	return opts
}

// diff holds the changes between two realms.
type diff struct {
	from, to *schema.Realm
	changes  []schema.Change
}

func computeDiff(ctx context.Context, differ *sqlclient.Client, from, to *cmdext.StateReadCloser, opts ...schema.DiffOption) (*diff, error) {
	current, err := from.ReadState(ctx)
	if err != nil {
		return nil, err
	}
	desired, err := to.ReadState(ctx)
	if err != nil {
		return nil, err
	}
	var changes []schema.Change
	switch {
	// In case an HCL file is compared against a specific database schema (not a realm).
	case to.HCL && len(desired.Schemas) == 1 && from.Schema != "" && desired.Schemas[0].Name != from.Schema:
		return nil, fmt.Errorf("mismatched HCL and database schemas: %q <> %q", from.Schema, desired.Schemas[0].Name)
	// Compare realm if the desired state is an HCL file or both connections are not bound to a schema.
	case from.HCL, to.HCL, from.Schema == "" && to.Schema == "":
		changes, err = differ.RealmDiff(current, desired, opts...)
		if err != nil {
			return nil, err
		}
	case from.Schema == "" && to.Schema != "":
		return nil, fmt.Errorf("cannot diff a schema %q with a database connection. See: https://atlasgo.io/url", to.Schema)
	case from.Schema != "" && to.Schema == "":
		return nil, fmt.Errorf("cannot diff a database connection with a schema %q. See: https://atlasgo.io/url", from.Schema)
	default:
		// SchemaDiff checks for name equality which is irrelevant in the case
		// the user wants to compare their contents, reset them to allow the comparison.
		current.Schemas[0].Name, desired.Schemas[0].Name = "", ""
		changes, err = differ.SchemaDiff(current.Schemas[0], desired.Schemas[0], opts...)
		if err != nil {
			return nil, err
		}
	}
	return &diff{
		changes: changes,
		from:    current,
		to:      desired,
	}, nil
}
