package standard

import (
	"ganigeorgiev.com/pocketbase/pocketbase"
	"ganigeorgiev.com/pocketbase/pocketbase/tools/osutils"
	"pocketone.dev/ext/config"
	"pocketone.dev/ext/jsvm"
)

// The JS engines a single app can run (`app: jsvm:` in pocketone.yaml).
const (
	JSEngineQuickJS = "quickjs" // ext/jsvm: PocketOne's engine, the default
	JSEngineGoja    = "goja"    // PocketBase's own plugins/jsvm, for pure PocketBase compatibility
)

// App is pocketone.yaml's app section turned into what a single PocketOne app is built from. Every binary that opens
// one app (the pocketone binary, a custom build, PocketHost's mothership) starts from it, so a key added to the app
// section reaches all of them without each binary copying the mapping. A binary's own flags then override the
// fields they cover.
//
//	cfg, err := config.Load(config.FromArgs(os.Args[1:]))
//	...
//	a, err := standard.AppFromConfig(cfg)
//	...
//	app := standard.New().NewApp(a.PocketBase, a.JS)
type App struct {
	// PocketBase is dir, dev, encryption_env and query_timeout as the defaults of PocketBase's own flags.
	PocketBase pocketbase.Config
	// JS is hooks_dir, migrations_dir, hooks_pool and js_limits (memory, task_time, task_wall, await_time).
	JS jsvm.Config
	// JSEngine is jsvm: JSEngineQuickJS (the default) or JSEngineGoja.
	JSEngine string
}

// AppFromConfig reads the app section of the config config.Load installed (with no config file, every key keeps its
// default). Relative paths resolve from the config file's folder, and only env_allow's names from the .env become
// readable by the app.
func AppFromConfig(cfg *config.Config) (App, error) {
	a, _ := config.App.Get()
	resolve := func(p string) string { return p }
	if cfg != nil {
		if err := cfg.AllowEnv(a.EnvAllow); err != nil {
			return App{}, err
		}
		resolve = cfg.ResolvePath
	}
	dev := osutils.IsProbablyGoRun()
	if a.Dev != nil {
		dev = *a.Dev
	}
	engine := a.JSVM
	if engine == "" {
		engine = JSEngineQuickJS
	}
	pool := a.HooksPool
	if pool == 0 {
		pool = 15
	}
	return App{
		PocketBase: pocketbase.Config{
			DefaultDev:           dev,
			DefaultDataDir:       resolve(a.Dir),
			DefaultEncryptionEnv: a.EncryptionEnv,
			DefaultQueryTimeout:  a.QueryTimeout,
		},
		JS: jsvm.Config{
			HooksDir:      resolve(a.HooksDir),
			MigrationsDir: resolve(a.MigrationsDir),
			HooksPoolSize: pool,
			VM: jsvm.VMConfig{ // 0 keeps each of ext/jsvm's defaults
				MemoryLimit: int64(a.JSLimits.Memory),
				TaskBudget:  a.JSLimits.TaskTime,
				TaskWall:    a.JSLimits.TaskWall,
				AwaitLimit:  a.JSLimits.AwaitTime,
			},
		},
		JSEngine: engine,
	}, nil
}
