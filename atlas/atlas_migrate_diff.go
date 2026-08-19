package atlas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"strconv"
	"text/template"
	"text/template/parse"
	"time"

	"github.com/lxt1045/atlas-cmd-internal/cmdext"
	cmdmigrate "github.com/lxt1045/atlas-cmd-internal/migrate"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlclient"
	"github.com/spf13/cobra"
)

// atlas migrate diff test_name --dir "file://e:/test/atlas/migrations" --to "file://e:/test/atlas/test.sql" --dev-url "mysql://root:BdtEnxxTnoN1luUR@10.1.1.121:3306/atlas_dev" --format '{{ sql . \"  \" }}'
func MigrateDiffRun(ctx context.Context, toURL, schemas []string, name, formatTo, dirURL, devURL, qualifier string) (err error) {
	dev, err := sqlclient.Open(ctx, devURL)
	if err != nil {
		return err
	}
	defer dev.Close()
	// Acquire a lock.
	unlock, err := dev.Lock(ctx, "atlas_migrate_diff", 10*time.Second)
	if err != nil {
		return fmt.Errorf("acquiring database lock: %w", err)
	}
	// If unlocking fails notify the user about it.
	defer func() { cobra.CheckErr(unlock()) }()
	// Open the migration directory.
	u, err := url.Parse(dirURL)
	if err != nil {
		return err
	}
	dir, err := cmdmigrate.DirURL(ctx, u, false)
	if err != nil {
		return err
	}
	var indent string
	f, err := cmdmigrate.Formatter(u)
	if err != nil {
		return err
	}
	if f, indent, err = mayIndent(u, f, formatTo); err != nil {
		return err
	}
	diffOpts := []schema.DiffOption{schema.DiffNormalized()} //diffOptions(cmd, env)
	// If there is a state-loader that requires a custom
	// 'migrate diff' handling, offload it the work.
	if d, ok := cmdext.States.Differ(toURL); ok {
		err := d.MigrateDiff(ctx, &cmdext.MigrateDiffOptions{
			To:      toURL,
			Name:    name,
			Indent:  indent,
			Dir:     dir,
			Dev:     dev,
			Options: diffOpts,
		})
		return maskNoPlan(err)
	}
	// Get a state reader for the desired state.
	desired, err := stateReader(ctx, &stateReaderConfig{
		urls:    toURL,
		dev:     dev,
		client:  dev,
		schemas: schemas,
		// vars:    env.Vars(),
	})
	if err != nil {
		return err
	}
	defer desired.Close()
	opts := []migrate.PlannerOption{
		migrate.PlanFormat(f),
		migrate.PlanWithIndent(indent),
		migrate.PlanWithDiffOptions(diffOpts...),
	}
	if dev.URL.Schema != "" {
		// Disable tables qualifier in schema-mode.
		opts = append(opts, migrate.PlanWithSchemaQualifier(qualifier))
	}
	// Plan the changes and create a new migration file.
	pl := migrate.NewPlanner(dev.Driver, dir, opts...)
	plan, err := func() (*migrate.Plan, error) {
		if dev.URL.Schema != "" {
			return pl.PlanSchema(ctx, name, desired.StateReader)
		}
		return pl.Plan(ctx, name, desired.StateReader)
	}()
	var cerr *migrate.NotCleanError
	switch {
	case errors.As(err, &cerr) && dev.URL.Schema == "" && desired.Schema != "":
		return fmt.Errorf("dev database is not clean (%s). Add a schema to the URL to limit the scope of the connection", cerr.Reason)
	case err != nil:
		return maskNoPlan(err)
	default:
		return pl.WritePlan(plan)
	}
}

// maskNoPlan masks ErrNoPlan errors.
func maskNoPlan(err error) error {
	if errors.Is(err, migrate.ErrNoPlan) {
		log.Println("The migration directory is synced with the desired state, no changes to be made")
		return nil
	}
	return err
}
func mayIndent(dir *url.URL, f migrate.Formatter, format string) (migrate.Formatter, string, error) {
	if format == "" {
		return f, "", nil
	}
	reject := errors.New(`'sql' can only be used to indent statements`)
	t, err := template.New("format").
		// The "sql" is a dummy function to detect if the
		// template was used to indent the SQL statements.
		Funcs(template.FuncMap{"sql": func(...any) (string, error) { return "", reject }}).
		Parse(format)
	if err != nil {
		return nil, "", fmt.Errorf("parse format: %w", err)
	}
	indent, ok := func() (string, bool) {
		if len(t.Tree.Root.Nodes) != 1 {
			return "", false
		}
		n, ok := t.Tree.Root.Nodes[0].(*parse.ActionNode)
		if !ok || len(n.Pipe.Cmds) != 1 || len(n.Pipe.Cmds[0].Args) < 2 || len(n.Pipe.Cmds[0].Args) > 3 {
			return "", false
		}
		args := n.Pipe.Cmds[0].Args
		if args[0].String() != "sql" || args[1].String() != "." && args[1].String() != "$" {
			return "", false
		}
		d := `""` // empty string as arg.
		if len(args) == 3 {
			d = args[2].String()
		}
		return d, true
	}()
	if ok {
		if indent, err = strconv.Unquote(indent); err != nil {
			return nil, "", fmt.Errorf("parse indent: %w", err)
		}
		return f, indent, nil
	}
	// If the template is not an indent, it cannot contain the "sql" function.
	if err := t.Execute(io.Discard, &migrate.Plan{}); err != nil && errors.Is(err, reject) {
		return nil, "", fmt.Errorf("%v. got: %v", reject, t.Root.String())
	}
	tfs := f.(migrate.TemplateFormatter)
	if len(tfs) != 1 {
		return nil, "", fmt.Errorf("cannot use format with: %q", dir.Query().Get("format"))
	}
	return migrate.TemplateFormatter{{N: tfs[0].N, C: t}}, "", nil
}
