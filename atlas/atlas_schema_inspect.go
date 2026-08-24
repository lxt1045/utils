package atlas

import (
	"context"
	"fmt"
	"os"
	"text/template"

	"github.com/lxt1045/atlas-cmd-internal/cmdlog"

	"ariga.io/atlas/sql/sqlclient"
	// _ "github.com/go-sql-driver/mysql"
)

// atlas schema inspect --from "mysql://root:password@127.0.0.1:3306/dji1" --to "file://e:/test/atlas/test.sql" --format '{{ sql . \"  \" }}' --dev-url "mysql://root:password@127.0.0.1:3306/atlas_dev"
func SchemaInspectRun(ctx context.Context, fromURL, schemas, exclude []string, formatTo, devURL string) (err error) {
	dev, err := sqlclient.Open(ctx, devURL)
	if err != nil {
		return err
	}
	defer dev.Close()

	r, err := stateReader(ctx, &stateReaderConfig{
		urls:    fromURL,
		dev:     dev,
		vars:    nil,
		schemas: schemas,
		exclude: exclude,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	client, ok := r.Closer.(*sqlclient.Client)
	if !ok && dev != nil {
		client = dev
	}
	format := cmdlog.SchemaInspectTemplate
	if v := formatTo; v != "" {
		if format, err = template.New("format").Funcs(cmdlog.InspectTemplateFuncs).Parse(v); err != nil {
			return fmt.Errorf("parse log format: %w", err)
		}
	}
	s, err := r.ReadState(ctx)
	if err != nil {
		return err
	}
	i := cmdlog.NewSchemaInspect(ctx, client, s)
	i.URL = fromURL[0]
	return format.Execute(os.Stdout, i)
}
