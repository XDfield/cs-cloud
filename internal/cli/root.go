package cli

import (
	"fmt"
	"os"
	"strings"

	"cs-cloud/internal/app"
	"cs-cloud/internal/platform"
)

func Execute() error {
	a, err := app.New()
	if err != nil {
		return err
	}

	parseGlobalFlags()

	return dispatch(a)
}

func parseGlobalFlags() {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--auth-path" && i+1 < len(args):
			platform.SetAuthPath(args[i+1])
			i++
		case len(args[i]) > 11 && args[i][:11] == "--auth-path=":
			platform.SetAuthPath(args[i][11:])
		}
	}
}

func commandArgs() []string {
	var rest []string
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--auth-path" && i+1 < len(args):
			i++
		case len(args[i]) > 11 && args[i][:11] == "--auth-path=":
		default:
			rest = append(rest, args[i])
		}
	}
	return rest
}

func parseMode() string {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case (args[i] == "--mode" || args[i] == "-m") && i+1 < len(args):
			return args[i+1]
		case strings.HasPrefix(args[i], "--mode="):
			return args[i][7:]
		case strings.HasPrefix(args[i], "-m="):
			return args[i][3:]
		}
	}
	return "cloud"
}

func dispatch(a *app.App) error {
	cmds := commandArgs()
	if len(cmds) == 0 {
		printUsage()
		return nil
	}

	switch cmds[0] {
	case "start":
		return start(a)
	case "stop":
		return stop(a)
	case "restart":
		return restart(a)
	case "status":
		return status(a)
	case "logs":
		return logs(a)
	case "doctor":
		return doctor(a)
	case "register":
		return register(a)
	case "login":
		return login(a)
	case "logout":
		return logout(a)
	case "version":
		printVersion()
		return nil
	case "update":
		return updateCmd(a)
	case "serve":
		return serve(a)
	case "_daemon":
		return runDaemon(a)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command: %s", cmds[0])
	}
}

func printUsage() {
	printTitle("cs-cloud")
	printSection("Usage")
	fmt.Println(dimStyle.Render("  cs-cloud [flags] <command>"))

	printSection("Flags")
	fmt.Print(renderKV([][2]string{
		{"--auth-path", "Path to auth.json (default: ~/.costrict/share/auth.json)"},
		{"--mode, -m", "Daemon mode: cloud (default) or local"},
	}))

	printSection("Commands")
	cmds := [][2]string{
		{"version", "Show version info"},
		{"update", "Manage updates (check, apply, rollback, history)"},
		{"start", "Start daemon (cloud mode with WS tunnel, or --mode local)"},
		{"stop", "Stop daemon"},
		{"restart", "Restart daemon"},
		{"status", "Show daemon status"},
		{"logs", "Show daemon logs"},
		{"doctor", "Show diagnostic info"},
		{"register", "Register device"},
		{"login", "Login via browser OAuth"},
		{"logout", "Delete credentials and device info"},
		{"serve", "Run server in foreground (no daemon)"},
	}
	fmt.Print(renderKV(cmds))
}
