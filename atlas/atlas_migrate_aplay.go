package atlas

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/lxt1045/atlas-cmd-internal/cmdapi"
	"github.com/lxt1045/atlas-cmd-internal/cmdlog"
	cmdmigrate "github.com/lxt1045/atlas-cmd-internal/migrate"
	"github.com/lxt1045/atlas-cmd-internal/migrate/ent/revision"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/schema"
	"ariga.io/atlas/sql/sqlclient"
	"github.com/spf13/cobra"
)

// atlas migrate apply --dir "file://path/to/migrations?format=golang-migrate" --url "mysql://..."
func MigrateApplyRun(ctx context.Context, dirFrom, urlTo string) (err error) {
	dirURL, err := url.Parse(dirFrom)
	if err != nil {
		return fmt.Errorf("parse dir-url: %w", err)
	}
	// Open and validate the migration directory.
	dir, err := cmdmigrate.DirURL(ctx, dirURL, false)
	if err != nil {
		return err
	}
	if err := migrate.Validate(dir); err != nil {
		return err
	}
	// Open a client to the database.
	if urlTo == "" {
		return errors.New(`required flag "url" not set`)
	}
	client, err := sqlclient.Open(ctx, urlTo)
	if err != nil {
		return err
	}
	defer client.Close()
	// Acquire a lock.
	unlock, err := client.Driver.Lock(ctx, applyLockValue, 10*time.Second)
	if err != nil {
		return fmt.Errorf("acquiring database lock: %w", err)
	}
	// If unlocking fails notify the user about it.
	defer func() { cobra.CheckErr(unlock()) }()
	if err := checkRevisionSchemaClarity(ctx, client, ""); err != nil {
		return err
	}
	var rrw migrate.RevisionReadWriter
	if rrw, err = entRevisions(ctx, client, ""); err != nil {
		return err
	}
	mrrw, ok := rrw.(cmdmigrate.RevisionReadWriter)
	if !ok {
		return fmt.Errorf("unexpected revision read-writer type: %T", rrw)
	}
	if err := mrrw.Migrate(ctx); err != nil {
		return err
	}
	// Setup reporting info.
	report := cmdlog.NewMigrateApply(ctx, client, dirURL)
	mr := &cmdapi.MigrateReport{}
	mr.Init(client, report, mrrw)
	// If cloud reporting is enabled, and we cannot obtain the current
	// target identifier, abort and report it to the user.
	if err := mr.RecordTargetID(ctx); err != nil {
		return err
	}
	// Determine pending files.
	opts, err := migrateOptions(false, "")
	if err != nil {
		return err
	}
	opts = append(opts, migrate.WithOperatorVersion(operatorVersion()), migrate.WithLogger(report))
	ex, err := migrate.NewExecutor(client.Driver, dir, rrw, opts...)
	if err != nil {
		return err
	}
	pending, err := ex.Pending(ctx)
	if err != nil && !errors.Is(err, migrate.ErrNoPendingFiles) {
		return err
	}
	noPending := errors.Is(err, migrate.ErrNoPendingFiles)
	// Get the pending files before obtaining applied revisions,
	// as the Executor may write a baseline revision in the table.
	applied, err := rrw.ReadRevisions(ctx)
	if err != nil {
		return err
	}
	if noPending {
		migrate.LogNoPendingFiles(report, applied)
		// return mr.Done(cmd, flags)
		return nil
	}
	count := len(pending)
	pending = pending[:count]
	migrate.LogIntro(report, applied, pending)
	var (
		mux = tx{
			dryRun: false,
			mode:   "file", // [none, file, all]: --tx-mode file (默认模式) 每个迁移文件作为一个独立的事务
			schema: "",     // 默认是： atlas_schema_revisions
			c:      client,
			rrw:    rrw,
		}
		drv migrate.Driver
	)
	for _, f := range pending {
		if drv, rrw, err = mux.driverFor(ctx, f); err != nil {
			break
		}
		if ex, err = migrate.NewExecutor(drv, dir, rrw, opts...); err != nil {
			return fmt.Errorf("unexpected executor creation error: %w", err)
		}
		if err = mux.mayRollback(ex.Execute(ctx, f)); err != nil {
			break
		}
		if err = mux.mayCommit(); err != nil {
			break
		}
	}
	if err == nil {
		if err = mux.commit(); err == nil {
			report.Log(migrate.LogDone{})
		}
	}
	if err != nil {
		report.Error = err.Error()
	}
	return err
}

func operatorVersion() string {
	return "swxc v0.01"
}

func migrateOptions(allowDirty bool, baseline string) ([]migrate.ExecutorOption, error) {
	var opts []migrate.ExecutorOption
	if allowDirty {
		opts = append(opts, migrate.WithAllowDirty(true))
	}
	if baseline != "" {
		opts = append(opts, migrate.WithBaselineVersion(baseline)) // 作为 Baseline 版本
	}

	// opts = append(opts, migrate.WithExecOrder(migrate.ExecOrderLinearSkip))  // 默认采用线性状态执行: 按版本号顺序执行
	// opts = append(opts, migrate.WithExecOrder(migrate.ExecOrderNonLinear))

	return opts, nil
}
func entRevisions(ctx context.Context, c *sqlclient.Client, flag string) (cmdmigrate.RevisionReadWriter, error) {
	return cmdmigrate.RevisionsForClient(ctx, c, revisionSchemaName(c, flag))
}

// defaultRevisionSchema is the default schema for storing revisions table.
const defaultRevisionSchema = "atlas_schema_revisions"

func revisionSchemaName(c *sqlclient.Client, flag string) string {
	switch {
	case flag != "":
		return flag
	case c.URL.Schema != "":
		return c.URL.Schema
	default:
		return defaultRevisionSchema
	}
}

const applyLockValue = "atlas_migrate_execute"

func checkRevisionSchemaClarity(ctx context.Context, c *sqlclient.Client, revisionSchemaFlag string) error {
	// The "old" default  behavior for the revision schema location was to store the revision table in its own schema.
	// Now, the table is saved in the connected schema, if any. To keep the backwards compatability, we now require
	// for schema bound connections to have the schema-revision flag present if there is no revision table in the schema
	// but the old default schema does have one.
	if c.URL.Schema != "" && revisionSchemaFlag == "" {
		// If the schema does not contain a revision table, but we can find a table in the previous default schema,
		// abort and tell the user to specify the intention.
		opts := &schema.InspectOptions{Tables: []string{revision.Table}, Mode: schema.InspectTables}
		s, err := c.InspectSchema(ctx, "", opts)
		var ok bool
		switch {
		case schema.IsNotExistError(err):
			// If the schema does not exist, the table does not as well.
		case err != nil:
			return err
		default:
			// Connected schema does exist, check if the table does.
			_, ok = s.Table(revision.Table)
		}
		if !ok { // Either schema or table does not exist.
			// Check for the old default schema. If it does not exist, we have no problem.
			s, err := c.InspectSchema(ctx, defaultRevisionSchema, opts)
			switch {
			case schema.IsNotExistError(err):
				// Schema does not exist, we can proceed.
			case err != nil:
				return err
			default:
				if _, ok := s.Table(revision.Table); ok {
					err := fmt.Errorf("ambiguous revision table: %s",
						`We couldn't find a revision table in the connected schema but found one in 
the schema 'atlas_schema_revisions' and cannot determine the desired behavior.

As a safety guard, we require you to specify whether to use the existing
table in 'atlas_schema_revisions' or create a new one in the connected schema
by providing the '--revisions-schema' flag or deleting the 'atlas_schema_revisions'
schema if it is unused.

`)
					return err
				}
			}
		}
	}
	return nil
}

const (
	txModeNone      = "none"
	txModeAll       = "all"
	txModeFile      = "file"
	txModeDirective = "txmode"

	execOrderLinear     = "linear"
	execOrderLinearSkip = "linear-skip"
	execOrderNonLinear  = "non-linear"
)

// tx handles wrapping migration execution in transactions.
type tx struct {
	dryRun       bool
	mode, schema string
	c            *sqlclient.Client
	rrw          migrate.RevisionReadWriter
	// current transaction context.
	tx    *sqlclient.TxClient
	txrrw migrate.RevisionReadWriter
}

// driverFor returns the migrate.Driver to use to execute migration statements.
func (tx *tx) driverFor(ctx context.Context, f migrate.File) (migrate.Driver, migrate.RevisionReadWriter, error) {
	if tx.dryRun {
		// If the --dry-run flag is given we don't want to execute any statements on the database.
		return &dryRunDriver{tx.c.Driver}, &dryRunRevisions{tx.rrw}, nil
	}
	mode, err := tx.modeFor(f)
	if err != nil {
		return nil, nil, err
	}
	switch mode {
	case txModeNone:
		return tx.c.Driver, tx.rrw, nil
	case txModeFile:
		// In file-mode, this function is called each time a new file is executed. Open a transaction.
		if tx.tx != nil {
			return nil, nil, errors.New("unexpected active transaction")
		}
		var err error
		tx.tx, err = tx.c.Tx(ctx, nil)
		if err != nil {
			return nil, nil, err
		}
		if tx.txrrw, err = entRevisions(ctx, tx.tx.Client, tx.schema); err != nil {
			return nil, nil, err
		}
		return tx.tx.Driver, tx.txrrw, nil
	case txModeAll:
		// In file-mode, this function is called each time a new file is executed. Since we wrap all files into one
		// huge transaction, if there already is an opened one, use that.
		if tx.tx == nil {
			var err error
			tx.tx, err = tx.c.Tx(ctx, nil)
			if err != nil {
				return nil, nil, err
			}
			if tx.txrrw, err = entRevisions(ctx, tx.tx.Client, tx.schema); err != nil {
				return nil, nil, err
			}
		}
		return tx.tx.Driver, tx.txrrw, nil
	default:
		return nil, nil, fmt.Errorf("unknown tx-mode %q", mode)
	}
}

// mayRollback may roll back a transaction depending on the given transaction mode.
func (tx *tx) mayRollback(err error) error {
	if tx.tx != nil && err != nil {
		if err2 := tx.tx.Rollback(); err2 != nil {
			err = fmt.Errorf("%v: %w", err2, err)
		}
	}
	return err
}

// mayCommit may commit a transaction depending on the given transaction mode.
func (tx *tx) mayCommit() error {
	// Only commit if each file is wrapped in a transaction.
	if tx.tx != nil && !tx.dryRun && tx.mode == txModeFile {
		return tx.commit()
	}
	return nil
}

// txmodeFor returns the transaction mode for the given file.
func txmodeFor(f *migrate.LocalFile) (string, error) {
	switch ds := f.Directive(txModeDirective); {
	case len(ds) == 0:
		return "", nil
	case len(ds) > 1:
		return "", fmt.Errorf("multiple txmode values found in file %q: %q", f.Name(), ds)
	case ds[0] == txModeAll:
		return "", fmt.Errorf("txmode %q is not allowed in file directive %q. Use %q instead", txModeAll, f.Name(), txModeFile)
	case ds[0] == txModeNone, ds[0] == txModeFile:
		return ds[0], nil
	default:
		return "", fmt.Errorf("unknown txmode %q found in file directive %q", ds[0], f.Name())
	}
}

// commit the transaction, if one is active.
func (tx *tx) commit() error {
	if tx.tx == nil {
		return nil
	}
	defer func() { tx.tx, tx.txrrw = nil, nil }()
	return tx.tx.Commit()
}

func (tx *tx) modeFor(f migrate.File) (string, error) {
	l, ok := f.(*migrate.LocalFile)
	if !ok {
		return tx.mode, nil
	}
	switch m, err := txmodeFor(l); {
	case err != nil:
		return "", err
	case m == "", m == tx.mode:
		return tx.mode, nil
	default: // m == txModeNone, m == txModeFile
		if tx.mode == txModeAll {
			return "", fmt.Errorf("cannot set txmode directive to %q in %q when txmode %q is set globally", m, l.Name(), txModeAll)
		}
		return m, nil
	}
}

type (
	// dryRunDriver wraps a migrate.Driver without executing any SQL statements.
	dryRunDriver struct{ migrate.Driver }

	// dryRunRevisions wraps a migrate.RevisionReadWriter without executing any SQL statements.
	dryRunRevisions struct{ migrate.RevisionReadWriter }
)

// ExecContext overrides the wrapped schema.ExecQuerier to not execute any SQL.
func (dryRunDriver) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}

// Lock implements the schema.Locker interface.
func (dryRunDriver) Lock(context.Context, string, time.Duration) (schema.UnlockFunc, error) {
	// We dry-run, we don't execute anything. Locking is not required.
	return func() error { return nil }, nil
}

// CheckClean implements the migrate.CleanChecker interface.
func (dryRunDriver) CheckClean(context.Context, *migrate.TableIdent) error {
	return nil
}

// Snapshot implements the migrate.Snapshoter interface.
func (dryRunDriver) Snapshot(context.Context) (migrate.RestoreFunc, error) {
	// We dry-run, we don't execute anything. Snapshotting not required.
	return func(context.Context) error { return nil }, nil
}

// WriteRevision overrides the wrapped migrate.RevisionReadWriter to not saved any changes to revisions.
func (dryRunRevisions) WriteRevision(context.Context, *migrate.Revision) error {
	return nil
}
