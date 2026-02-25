package bootstrap

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	collabinadapter "mdht/internal/modules/collab/adapter/in"
	collaboutadapter "mdht/internal/modules/collab/adapter/out"
	collabservice "mdht/internal/modules/collab/service"
	collabusecase "mdht/internal/modules/collab/usecase"
	graphinadapter "mdht/internal/modules/graph/adapter/in"
	graphoutadapter "mdht/internal/modules/graph/adapter/out"
	graphservice "mdht/internal/modules/graph/service"
	graphusecase "mdht/internal/modules/graph/usecase"
	libraryinadapter "mdht/internal/modules/library/adapter/in"
	libraryoutadapter "mdht/internal/modules/library/adapter/out"
	libraryservice "mdht/internal/modules/library/service"
	libraryusecase "mdht/internal/modules/library/usecase"
	plugininadapter "mdht/internal/modules/plugin/adapter/in"
	pluginoutadapter "mdht/internal/modules/plugin/adapter/out"
	pluginservice "mdht/internal/modules/plugin/service"
	pluginusecase "mdht/internal/modules/plugin/usecase"
	readercliadapter "mdht/internal/modules/reader/adapter/in"
	readeroutadapter "mdht/internal/modules/reader/adapter/out"
	readerservice "mdht/internal/modules/reader/service"
	readerusecase "mdht/internal/modules/reader/usecase"
	sessioninadapter "mdht/internal/modules/session/adapter/in"
	sessionoutadapter "mdht/internal/modules/session/adapter/out"
	sessionservice "mdht/internal/modules/session/service"
	sessionusecase "mdht/internal/modules/session/usecase"
	"mdht/internal/platform/clock"
	"mdht/internal/platform/config"
	"mdht/internal/platform/id"
	uiapp "mdht/internal/ui/app"
)

type App struct {
	LibraryCLI libraryinadapter.CLIHandler
	SessionCLI sessioninadapter.CLIHandler
	ReaderCLI  readercliadapter.CLIHandler
	ReaderTUI  readercliadapter.TUIHandler
	GraphCLI   graphinadapter.CLIHandler
	PluginCLI  plugininadapter.CLIHandler
	CollabCLI  collabinadapter.CLIHandler
}

func New(cfg config.Config) (*App, error) {
	clk := clock.SystemClock{}
	ids := id.RandomHex{}

	graphTopicStore := graphoutadapter.NewVaultTopicStore(cfg.VaultPath)
	graphLinkProjector, err := graphoutadapter.NewSQLiteLinkProjector(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("new graph projector: %w", err)
	}
	graphSvc := graphservice.NewGraphService(graphTopicStore, graphLinkProjector)
	graphUC := graphusecase.NewInteractor(graphSvc)

	libraryStore := libraryoutadapter.NewVaultSourceStore(cfg.VaultPath)
	libraryProjector, err := libraryoutadapter.NewSQLiteSourceProjector(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("new source projector: %w", err)
	}
	librarySvc := libraryservice.NewSourceService(clk, ids, libraryStore, libraryProjector)
	libraryUC := libraryusecase.NewInteractor(librarySvc, graphUC)

	activeStore := sessionoutadapter.NewFileActiveSessionStore(cfg.VaultPath)
	sessionUC := sessionusecase.NewInteractor(
		sessionservice.NewSessionService(clk, ids, sessionoutadapter.NewVaultSessionStore(cfg.VaultPath)),
		libraryUC,
		activeStore,
	)

	readerUC := readerusecase.NewInteractor(readerservice.NewReaderService(
		readeroutadapter.NewLocalMarkdownReader(),
		readeroutadapter.NewLocalPDFReader(),
		readeroutadapter.NewLibraryProgressAdapter(libraryUC),
		readeroutadapter.NewLibrarySourceAdapter(libraryUC),
		readeroutadapter.NewOSExternalLauncher(),
	))

	pluginUC := pluginusecase.NewInteractor(pluginservice.NewPluginService(
		pluginoutadapter.NewFileManifestStore(cfg.VaultPath),
		pluginoutadapter.NewGRPCHost(),
	))
	collabBridge := collaboutadapter.NewVaultProjectionBridge(cfg.VaultPath)
	collabUC := collabusecase.NewInteractor(collabservice.NewCollabService(
		cfg.VaultPath,
		collaboutadapter.NewFileWorkspaceStore(cfg.VaultPath),
		collaboutadapter.NewFilePeerStore(cfg.VaultPath),
		collaboutadapter.NewFileOpLogStore(cfg.VaultPath),
		collaboutadapter.NewFileSnapshotStore(cfg.VaultPath),
		collabBridge,
		collabBridge,
		collaboutadapter.NewLibp2pTransport(),
		collaboutadapter.NewFileDaemonStore(cfg.VaultPath),
		collaboutadapter.NewJSONRPCServer(),
		collaboutadapter.NewJSONRPCClient(),
	))

	return &App{
		LibraryCLI: libraryinadapter.NewCLIHandler(libraryUC),
		SessionCLI: sessioninadapter.NewCLIHandler(sessionUC),
		ReaderCLI:  readercliadapter.NewCLIHandler(readerUC),
		ReaderTUI:  readercliadapter.NewTUIHandler(readerUC),
		GraphCLI:   graphinadapter.NewCLIHandler(graphUC),
		PluginCLI:  plugininadapter.NewCLIHandler(pluginUC),
		CollabCLI:  collabinadapter.NewCLIHandler(collabUC),
	}, nil
}

func RunTUI(vaultPath string, app *App) error {
	model := uiapp.NewModel(vaultPath, app.LibraryCLI, app.SessionCLI, app.ReaderTUI, app.PluginCLI)
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err := program.Run()
	return err
}
