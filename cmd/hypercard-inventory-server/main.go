package main

import (
	"context"
	"embed"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	clay "github.com/go-go-golems/clay/pkg"
	gepprofiles "github.com/go-go-golems/geppetto/pkg/profiles"
	geppettosections "github.com/go-go-golems/geppetto/pkg/sections"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/logging"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/help"
	help_cmd "github.com/go-go-golems/glazed/pkg/help/cmd"
	webchat "github.com/go-go-golems/pinocchio/pkg/webchat"
	webhttp "github.com/go-go-golems/pinocchio/pkg/webchat/http"
	plzconfirmbackend "github.com/go-go-golems/plz-confirm/pkg/backend"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/go-go-golems/hypercard-inventory-chat/internal/inventorydb"
	"github.com/go-go-golems/hypercard-inventory-chat/internal/pinoweb"
)

//go:embed static
var staticFS embed.FS

type Command struct {
	*cmds.CommandDescription
}

type serverSettings struct {
	Root                 string `glazed:"root"`
	InventoryDB          string `glazed:"inventory-db"`
	InventorySeedOnStart bool   `glazed:"inventory-seed-on-start"`
	InventoryResetOnBoot bool   `glazed:"inventory-reset-on-start"`
}

const (
	profileRegistrySlug = "default"
)

func NewCommand() (*Command, error) {
	geLayers, err := geppettosections.CreateGeppettoSections()
	if err != nil {
		return nil, errors.Wrap(err, "create geppetto sections")
	}

	desc := cmds.NewCommandDescription(
		"hypercard-inventory-server",
		cmds.WithShort("Serve inventory chat endpoints using Pinocchio webchat"),
		cmds.WithFlags(
			fields.New("addr", fields.TypeString, fields.WithDefault(":8091"), fields.WithHelp("HTTP listen address")),
			fields.New("root", fields.TypeString, fields.WithDefault("/"), fields.WithHelp("Serve handlers under a URL root (for example /chat)")),
			fields.New("idle-timeout-seconds", fields.TypeInteger, fields.WithDefault(60), fields.WithHelp("Stop per-conversation reader after N seconds with no sockets (0=disabled)")),
			fields.New("evict-idle-seconds", fields.TypeInteger, fields.WithDefault(300), fields.WithHelp("Evict conversations after N seconds idle (0=disabled)")),
			fields.New("evict-interval-seconds", fields.TypeInteger, fields.WithDefault(60), fields.WithHelp("Sweep idle conversations every N seconds (0=disabled)")),
			fields.New("timeline-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable timeline snapshots (preferred over timeline-db)")),
			fields.New("timeline-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable timeline snapshots")),
			fields.New("turns-dsn", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DSN for durable turn snapshots (preferred over turns-db)")),
			fields.New("turns-db", fields.TypeString, fields.WithDefault(""), fields.WithHelp("SQLite DB file path for durable turn snapshots")),
			fields.New("timeline-inmem-max-entities", fields.TypeInteger, fields.WithDefault(1000), fields.WithHelp("In-memory timeline entity cap when no timeline DB is configured")),
			fields.New("inventory-db", fields.TypeString, fields.WithDefault("./data/inventory.db"), fields.WithHelp("SQLite DB file path for inventory domain data")),
			fields.New("inventory-seed-on-start", fields.TypeBool, fields.WithDefault(true), fields.WithHelp("Seed inventory domain data during startup")),
			fields.New("inventory-reset-on-start", fields.TypeBool, fields.WithDefault(false), fields.WithHelp("Reset inventory domain tables before seeding")),
		),
		cmds.WithSections(geLayers...),
	)

	return &Command{CommandDescription: desc}, nil
}

func (c *Command) RunIntoWriter(ctx context.Context, parsed *values.Values, _ io.Writer) error {
	cfg := &serverSettings{}
	_ = parsed.DecodeSectionInto(values.DefaultSlug, cfg)

	inventoryDBPath := strings.TrimSpace(cfg.InventoryDB)
	if inventoryDBPath == "" {
		inventoryDBPath = "./data/inventory.db"
	}
	sqliteDB, err := inventorydb.Open(inventoryDBPath)
	if err != nil {
		return errors.Wrap(err, "open inventory sqlite db")
	}
	defer func() { _ = sqliteDB.Close() }()

	if err := inventorydb.Migrate(sqliteDB); err != nil {
		return errors.Wrap(err, "migrate inventory sqlite db")
	}
	if cfg.InventoryResetOnBoot {
		if err := inventorydb.ResetAndSeed(sqliteDB); err != nil {
			return errors.Wrap(err, "reset and seed inventory sqlite db")
		}
	} else if cfg.InventorySeedOnStart {
		if err := inventorydb.Seed(sqliteDB); err != nil {
			return errors.Wrap(err, "seed inventory sqlite db")
		}
	}

	inventoryStore, err := inventorydb.NewStore(sqliteDB)
	if err != nil {
		return errors.Wrap(err, "create inventory store")
	}

	composer := pinoweb.NewRuntimeComposer(parsed, pinoweb.RuntimeComposerOptions{
		RuntimeKey:   "inventory",
		SystemPrompt: "You are an inventory assistant. Be concise, accurate, and tool-first.",
		AllowedTools: append([]string(nil), inventoryToolNames...),
	})
	pinoweb.RegisterInventoryHypercardExtensions()
	profileRegistry, err := newInMemoryProfileService(
		"default",
		&gepprofiles.Profile{
			Slug:        gepprofiles.MustProfileSlug("default"),
			DisplayName: "Default",
			Description: "Baseline assistant profile with no app-specific middleware configuration.",
			Runtime: gepprofiles.RuntimeSpec{
				SystemPrompt: "You are a helpful inventory assistant.",
				Middlewares:  []gepprofiles.MiddlewareUse{},
			},
		},
		&gepprofiles.Profile{
			Slug:        gepprofiles.MustProfileSlug("inventory"),
			DisplayName: "Inventory",
			Description: "Tool-first inventory assistant profile.",
			Runtime: gepprofiles.RuntimeSpec{
				SystemPrompt: "You are an inventory assistant. Be concise, accurate, and tool-first.",
				Middlewares:  inventoryRuntimeMiddlewares(),
				Tools:        append([]string(nil), inventoryToolNames...),
			},
		},
		&gepprofiles.Profile{
			Slug:        gepprofiles.MustProfileSlug("analyst"),
			DisplayName: "Analyst",
			Description: "Analysis-oriented profile for inventory reporting tasks.",
			Runtime: gepprofiles.RuntimeSpec{
				SystemPrompt: "You are an inventory analyst. Explain results with concise evidence.",
				Middlewares:  inventoryRuntimeMiddlewares(),
				Tools:        append([]string(nil), inventoryToolNames...),
			},
		},
		&gepprofiles.Profile{
			Slug:        gepprofiles.MustProfileSlug("planner"),
			DisplayName: "Planner",
			Description: "Planning-focused profile for restock and operations scenarios.",
			Runtime: gepprofiles.RuntimeSpec{
				SystemPrompt: "You are an inventory operations planner. Prioritize actionable next steps.",
				Middlewares:  inventoryRuntimeMiddlewares(),
				Tools:        append([]string(nil), inventoryToolNames...),
			},
		},
	)
	if err != nil {
		return errors.Wrap(err, "initialize profile registry")
	}
	requestResolver := pinoweb.NewStrictRequestResolver("inventory").WithProfileRegistry(
		profileRegistry,
		gepprofiles.MustRegistrySlug(profileRegistrySlug),
	)

	srv, err := webchat.NewServer(
		ctx,
		parsed,
		staticFS,
		webchat.WithRuntimeComposer(composer),
		webchat.WithEventSinkWrapper(pinoweb.NewInventoryEventSinkWrapper(ctx)),
		webchat.WithDebugRoutesEnabled(os.Getenv("PINOCCHIO_WEBCHAT_DEBUG") == "1"),
	)
	if err != nil {
		return errors.Wrap(err, "new webchat server")
	}
	for name, factory := range inventoryToolFactories(inventoryStore) {
		srv.RegisterTool(name, factory)
	}

	chatHandler := webhttp.NewChatHandler(srv.ChatService(), requestResolver)
	wsHandler := webhttp.NewWSHandler(
		srv.StreamHub(),
		requestResolver,
		websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	)
	timelineHandler := webhttp.NewTimelineHandler(srv.TimelineService(), log.With().Str("component", "inventory-chat").Str("route", "/api/timeline").Logger())

	appMux := http.NewServeMux()
	appMux.HandleFunc("/chat", chatHandler)
	appMux.HandleFunc("/chat/", chatHandler)
	appMux.HandleFunc("/ws", wsHandler)
	webhttp.RegisterProfileAPIHandlers(appMux, profileRegistry, webhttp.ProfileAPIHandlerOptions{
		DefaultRegistrySlug:             gepprofiles.MustRegistrySlug(profileRegistrySlug),
		EnableCurrentProfileCookieRoute: true,
		WriteActor:                      "hypercard-inventory-server",
		WriteSource:                     "http-api",
		MiddlewareDefinitions:           composer.MiddlewareDefinitions(),
		ExtensionSchemas: []webhttp.ExtensionSchemaDocument{
			{
				Key: "inventory.starter_suggestions@v1",
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"default": []any{},
						},
					},
					"required":             []any{"items"},
					"additionalProperties": false,
				},
			},
		},
	})
	appMux.HandleFunc("/api/timeline", timelineHandler)
	appMux.HandleFunc("/api/timeline/", timelineHandler)
	appMux.Handle("/api/", srv.APIHandler())
	plzconfirmbackend.NewServer().Mount(appMux, "/confirm")
	appMux.Handle("/", srv.UIHandler())

	httpSrv := srv.HTTPServer()
	if httpSrv == nil {
		return errors.New("http server is not initialized")
	}

	if cfg.Root != "" && cfg.Root != "/" {
		parent := http.NewServeMux()
		prefix := cfg.Root
		if !strings.HasPrefix(prefix, "/") {
			prefix = "/" + prefix
		}
		if !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		parent.Handle(prefix, http.StripPrefix(strings.TrimRight(prefix, "/"), appMux))
		httpSrv.Handler = parent
	} else {
		httpSrv.Handler = appMux
	}

	return srv.Run(ctx)
}

func newInMemoryProfileService(defaultSlug string, profileDefs ...*gepprofiles.Profile) (gepprofiles.Registry, error) {
	registrySlug := gepprofiles.MustRegistrySlug(profileRegistrySlug)
	registry := &gepprofiles.ProfileRegistry{
		Slug:     registrySlug,
		Profiles: map[gepprofiles.ProfileSlug]*gepprofiles.Profile{},
	}

	for _, profile := range profileDefs {
		if profile == nil {
			continue
		}
		clone := profile.Clone()
		if clone == nil {
			continue
		}
		if err := gepprofiles.ValidateProfile(clone); err != nil {
			return nil, err
		}
		registry.Profiles[clone.Slug] = clone
	}

	if strings.TrimSpace(defaultSlug) != "" {
		slug, err := gepprofiles.ParseProfileSlug(defaultSlug)
		if err != nil {
			return nil, err
		}
		registry.DefaultProfileSlug = slug
	}

	if len(registry.Profiles) > 0 {
		if registry.DefaultProfileSlug.IsZero() {
			registry.DefaultProfileSlug = firstProfileSlug(registry.Profiles)
		}
		if _, ok := registry.Profiles[registry.DefaultProfileSlug]; !ok {
			registry.DefaultProfileSlug = firstProfileSlug(registry.Profiles)
		}
	}

	if err := gepprofiles.ValidateRegistry(registry); err != nil {
		return nil, err
	}
	store := gepprofiles.NewInMemoryProfileStore()
	if err := store.UpsertRegistry(context.Background(), registry, gepprofiles.SaveOptions{
		Actor:  "hypercard-inventory-server",
		Source: "builtin",
	}); err != nil {
		return nil, err
	}
	return gepprofiles.NewStoreRegistry(store, registrySlug)
}

func firstProfileSlug(profiles map[gepprofiles.ProfileSlug]*gepprofiles.Profile) gepprofiles.ProfileSlug {
	slugs := make([]gepprofiles.ProfileSlug, 0, len(profiles))
	for slug := range profiles {
		slugs = append(slugs, slug)
	}
	sort.Slice(slugs, func(i, j int) bool { return slugs[i] < slugs[j] })
	if len(slugs) == 0 {
		return ""
	}
	return slugs[0]
}

func inventoryRuntimeMiddlewares() []gepprofiles.MiddlewareUse {
	return []gepprofiles.MiddlewareUse{
		{Name: "inventory_artifact_policy", ID: "artifact-policy"},
		{Name: "inventory_suggestions_policy", ID: "suggestions-policy"},
	}
}

func main() {
	root := &cobra.Command{
		Use: "hypercard-inventory-server",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return logging.InitLoggerFromCobra(cmd)
		},
	}

	helpSystem := help.NewHelpSystem()
	help_cmd.SetupCobraRootCommand(helpSystem, root)

	if err := clay.InitGlazed("hypercard-inventory-chat", root); err != nil {
		cobra.CheckErr(err)
	}

	c, err := NewCommand()
	cobra.CheckErr(err)
	command, err := cli.BuildCobraCommand(c, cli.WithCobraMiddlewaresFunc(geppettosections.GetCobraCommandGeppettoMiddlewares))
	cobra.CheckErr(err)
	root.AddCommand(command)
	cobra.CheckErr(root.Execute())
}
